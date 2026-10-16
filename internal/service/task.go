package service

import (
	"context"
	"example.com/task-tracker-api/internal/domain"
	"fmt"
	"strings"
	"unicode/utf8"
)

// TaskRepository describes storage operations needed by the service.
// Returning values keeps callers from mutating stored tasks outside the repository.
type TaskRepository interface {
	Create(context.Context, domain.Task) (domain.Task, error)
	Get(context.Context, string) (domain.Task, error)
	List(context.Context) ([]domain.Task, error)
	Complete(context.Context, string) (domain.Task, error)
	Delete(context.Context, string) error
}

type TaskService struct{ repo TaskRepository }

func NewTaskService(repo TaskRepository) *TaskService { return &TaskService{repo: repo} }

func (s *TaskService) Create(ctx context.Context, title, description string) (domain.Task, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n == 0 || n > 200 {
		return domain.Task{}, fmt.Errorf("%w: title must contain 1 to 200 characters", domain.ErrInvalidArgument)
	}
	if utf8.RuneCountInString(description) > 2000 {
		return domain.Task{}, fmt.Errorf("%w: description must contain at most 2000 characters", domain.ErrInvalidArgument)
	}
	return s.repo.Create(ctx, domain.Task{Title: title, Description: description})
}
func validateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidArgument)
	}
	return nil
}
func (s *TaskService) Get(ctx context.Context, id string) (domain.Task, error) {
	if err := validateID(id); err != nil {
		return domain.Task{}, err
	}
	return s.repo.Get(ctx, id)
}
func (s *TaskService) List(ctx context.Context) ([]domain.Task, error) { return s.repo.List(ctx) }
func (s *TaskService) Complete(ctx context.Context, id string) (domain.Task, error) {
	if err := validateID(id); err != nil {
		return domain.Task{}, err
	}
	return s.repo.Complete(ctx, id)
}
func (s *TaskService) Delete(ctx context.Context, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}
