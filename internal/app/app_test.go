package app_test

import (
	"context"
	"encoding/json"
	taskv1 "example.com/task-tracker-api/gen/task/v1"
	"example.com/task-tracker-api/internal/app"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGatewayAndGRPCLifecycle(t *testing.T) {
	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer grpcListener.Close()
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer httpListener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- app.Serve(ctx, grpcListener, httpListener) }()
	defer func() {
		cancel()
		select {
		case err := <-stopped:
			if err != nil {
				t.Errorf("shutdown: %v", err)
			}
		case <-time.After(7 * time.Second):
			t.Error("shutdown timed out")
		}
	}()
	conn, err := grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := taskv1.NewTaskServiceClient(conn)
	requestCtx, requestCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer requestCancel()
	initial, err := client.ListTasks(requestCtx, &taskv1.ListTasksRequest{}, grpc.WaitForReady(true))
	if err != nil || len(initial.GetTasks()) != 0 {
		t.Fatalf("initial list: %v %v", initial, err)
	}
	httpClient := &http.Client{Timeout: 3 * time.Second}
	defer httpClient.CloseIdleConnections()
	call := func(method, path, body string, want int) map[string]any {
		t.Helper()
		req, err := http.NewRequest(method, "http://"+httpListener.Addr().String()+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Fatalf("%s %s: status=%d body=%s want=%d", method, path, resp.StatusCode, data, want)
		}
		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("JSON %s: %v", data, err)
		}
		return result
	}
	empty := call("GET", "/v1/tasks", "", 200)
	if tasks, ok := empty["tasks"].([]any); !ok || len(tasks) != 0 {
		t.Fatalf("empty JSON list: %v", empty)
	}
	call("POST", "/v1/tasks", `{"title":"   "}`, 400)
	call("POST", "/v1/tasks", `{`, 400)
	created := call("POST", "/v1/tasks", `{"title":"  Изучить слои  ","description":"HTTP → gRPC"}`, 200)
	id, ok := created["id"].(string)
	if !ok || id == "" || created["title"] != "Изучить слои" || created["completed"] != false {
		t.Fatalf("created: %v", created)
	}
	task, err := client.GetTask(requestCtx, &taskv1.GetTaskRequest{Id: id})
	if err != nil || task.GetDescription() != "HTTP → gRPC" {
		t.Fatalf("cross-transport read: %v %v", task, err)
	}
	for range 2 {
		completed := call("POST", "/v1/tasks/"+id+"/complete", "", 200)
		if completed["completed"] != true {
			t.Fatalf("complete: %v", completed)
		}
	}
	task, err = client.GetTask(requestCtx, &taskv1.GetTaskRequest{Id: id})
	if err != nil || !task.GetCompleted() {
		t.Fatalf("completion not stored: %v %v", task, err)
	}
	call("GET", "/v1/tasks/"+id, "", 200)
	call("DELETE", "/v1/tasks/"+id, "", 200)
	call("GET", "/v1/tasks/"+id, "", 404)
	call("POST", "/v1/tasks/"+id+"/complete", "", 404)
	call("DELETE", "/v1/tasks/"+id, "", 404)
	_, err = client.GetTask(requestCtx, &taskv1.GetTaskRequest{Id: id})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("deleted gRPC task: %v", err)
	}
	_, err = client.GetTask(requestCtx, &taskv1.GetTaskRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty gRPC id: %v", err)
	}

	// Concurrent network clients exercise shared repository access under the race detector.
	var wg sync.WaitGroup
	ids := make(chan string, 24)
	for i := range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := client.CreateTask(requestCtx, &taskv1.CreateTaskRequest{Title: fmt.Sprintf("task %d", i)})
			if err != nil {
				t.Errorf("concurrent create: %v", err)
				return
			}
			ids <- task.GetId()
			if _, err := client.CompleteTask(requestCtx, &taskv1.CompleteTaskRequest{Id: task.GetId()}); err != nil {
				t.Errorf("concurrent complete: %v", err)
			}
			if _, err := client.ListTasks(requestCtx, &taskv1.ListTasksRequest{}); err != nil {
				t.Errorf("concurrent list: %v", err)
			}
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate ID %s", id)
		}
		seen[id] = true
	}
	list, err := client.ListTasks(requestCtx, &taskv1.ListTasksRequest{})
	if err != nil || len(list.GetTasks()) != 24 {
		t.Fatalf("concurrent results: %v %v", list, err)
	}
	for _, task := range list.GetTasks() {
		if !task.GetCompleted() {
			t.Errorf("lost update: %v", task)
		}
	}
}
