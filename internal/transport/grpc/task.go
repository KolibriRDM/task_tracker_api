// Package grpc adapts protobuf requests to the application service.
package grpc

import (
	"context"
	"errors"
	taskv1 "example.com/task-tracker-api/gen/task/v1"
	"example.com/task-tracker-api/internal/domain"
	"example.com/task-tracker-api/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"log/slog"
)

type TaskHandler struct {
	taskv1.UnimplementedTaskServiceServer
	service *service.TaskService
}

func NewTaskHandler(s *service.TaskService) *TaskHandler { return &TaskHandler{service: s} }
func toProto(task domain.Task) *taskv1.Task {
	return &taskv1.Task{Id: task.ID, Title: task.Title, Description: task.Description, Completed: task.Completed}
}
func toStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		slog.Error("task operation failed", "error", err)
		return status.Error(codes.Internal, "internal server error")
	}
}
func (h *TaskHandler) CreateTask(ctx context.Context, req *taskv1.CreateTaskRequest) (*taskv1.Task, error) {
	task, err := h.service.Create(ctx, req.GetTitle(), req.GetDescription())
	if err != nil {
		return nil, toStatus(err)
	}
	return toProto(task), nil
}
func (h *TaskHandler) GetTask(ctx context.Context, req *taskv1.GetTaskRequest) (*taskv1.Task, error) {
	task, err := h.service.Get(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return toProto(task), nil
}
func (h *TaskHandler) ListTasks(ctx context.Context, req *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
	tasks, err := h.service.List(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	result := &taskv1.ListTasksResponse{Tasks: make([]*taskv1.Task, 0, len(tasks))}
	for _, task := range tasks {
		result.Tasks = append(result.Tasks, toProto(task))
	}
	return result, nil
}
func (h *TaskHandler) CompleteTask(ctx context.Context, req *taskv1.CompleteTaskRequest) (*taskv1.Task, error) {
	task, err := h.service.Complete(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return toProto(task), nil
}
func (h *TaskHandler) DeleteTask(ctx context.Context, req *taskv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := h.service.Delete(ctx, req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &emptypb.Empty{}, nil
}
