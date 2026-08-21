// Package domain defines the operator-owned Project grouping boundary.
package domain

import (
	"context"
	"time"

	"sre-kit/internal/platform/apierror"
)

type Project struct {
	ID          string
	Name        string
	Slug        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var ErrNotFound = apierror.NotFound("project not found")

type Repository interface {
	Create(context.Context, Project) error
	Update(context.Context, Project) error
	Get(context.Context, string) (Project, error)
	List(context.Context) ([]Project, error)
	Delete(context.Context, string) error
}
