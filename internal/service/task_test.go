package service_test

import (
	"context"
	"errors"
	"example.com/task-tracker-api/internal/domain"
	"example.com/task-tracker-api/internal/repository/memory"
	"example.com/task-tracker-api/internal/service"
	"strings"
	"testing"
)

func TestValidationAndCancellation(t *testing.T) {
	s := service.NewTaskService(memory.NewTaskRepository())
	ctx := context.Background()
	for _, title := range []string{"", " \n\t", strings.Repeat("я", 201)} {
		if _, err := s.Create(ctx, title, ""); !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("title validation: %v", err)
		}
	}
	if _, err := s.Create(ctx, "valid", strings.Repeat("я", 2001)); !errors.Is(err, domain.ErrInvalidArgument) {
		t.Errorf("description validation: %v", err)
	}
	valid, err := s.Create(ctx, strings.Repeat("я", 200), strings.Repeat("я", 2000))
	if err != nil {
		t.Fatal(err)
	}
	valid.Title = "caller changed copy"
	stored, err := s.Get(ctx, valid.ID)
	if err != nil || stored.Title == valid.Title {
		t.Fatalf("storage copy: %v %v", stored, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Create(canceled, "new", ""); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled create: %v", err)
	}
	if err := s.Delete(canceled, valid.ID); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled delete: %v", err)
	}
	tasks, err := s.List(ctx)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("cancellation mutated storage: %v %v", tasks, err)
	}
}
