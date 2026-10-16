// Package memory implements volatile, concurrency-safe task storage.
package memory

import (
	"context"
	"example.com/task-tracker-api/internal/domain"
	"sort"
	"strconv"
	"sync"
)

type TaskRepository struct {
	mu     sync.RWMutex
	tasks  map[string]domain.Task
	nextID uint64
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{tasks: make(map[string]domain.Task)}
}
func (r *TaskRepository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Task{}, err
	}
	r.nextID++
	task.ID = strconv.FormatUint(r.nextID, 10)
	r.tasks[task.ID] = task
	return task, nil
}
func (r *TaskRepository) Get(ctx context.Context, id string) (domain.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return domain.Task{}, err
	}
	task, ok := r.tasks[id]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	return task, nil
}
func (r *TaskRepository) List(ctx context.Context) ([]domain.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tasks := make([]domain.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		a, _ := strconv.ParseUint(tasks[i].ID, 10, 64)
		b, _ := strconv.ParseUint(tasks[j].ID, 10, 64)
		return a < b
	})
	return tasks, nil
}

// Complete performs an atomic update. Repeating it leaves the task completed.
func (r *TaskRepository) Complete(ctx context.Context, id string) (domain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Task{}, err
	}
	task, ok := r.tasks[id]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	task.Completed = true
	r.tasks[id] = task
	return task, nil
}
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := r.tasks[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.tasks, id)
	return nil
}
