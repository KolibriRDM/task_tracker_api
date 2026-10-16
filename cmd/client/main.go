// This optional client demonstrates a direct gRPC call; it is not part of the server.
package main

import (
	"context"
	taskv1 "example.com/task-tracker-api/gen/task/v1"
	"flag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:50051", "gRPC server address")
	flag.Parse()
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := taskv1.NewTaskServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := client.ListTasks(ctx, &taskv1.ListTasksRequest{})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Tasks: %d", len(result.GetTasks()))
	for _, task := range result.GetTasks() {
		log.Printf("id=%s title=%q completed=%t", task.GetId(), task.GetTitle(), task.GetCompleted())
	}
}
