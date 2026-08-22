package application

import (
	"context"
	"testing"

	"sre-kit/internal/projects/domain"
)

type deleteTrackingRepository struct {
	deleted bool
}

func (r *deleteTrackingRepository) Create(context.Context, domain.Project) error { return nil }
func (r *deleteTrackingRepository) Update(context.Context, domain.Project) error { return nil }
func (r *deleteTrackingRepository) Get(context.Context, string) (domain.Project, error) {
	return domain.Project{}, nil
}
func (r *deleteTrackingRepository) List(context.Context) ([]domain.Project, error) { return nil, nil }
func (r *deleteTrackingRepository) Delete(context.Context, string) error {
	r.deleted = true
	return nil
}

func TestDeleteRejectsDefaultProjectBeforeRepositoryMutation(t *testing.T) {
	repository := &deleteTrackingRepository{}
	service := NewService(repository)

	err := service.Delete(context.Background(), "default")

	if err == nil {
		t.Fatal("Delete(default): expected an error")
	}
	if repository.deleted {
		t.Fatal("Delete(default): repository deletion must not be attempted")
	}
}
