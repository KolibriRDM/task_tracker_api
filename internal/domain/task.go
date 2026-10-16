// Package domain contains application data and errors, independent of transport and storage.
package domain

import "errors"

type Task struct {
	ID          string
	Title       string
	Description string
	Completed   bool
}

var (
	ErrNotFound        = errors.New("task not found")
	ErrInvalidArgument = errors.New("invalid argument")
)
