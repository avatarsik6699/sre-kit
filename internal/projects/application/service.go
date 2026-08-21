// Package application implements Project CRUD without importing infrastructure details.
package application

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"sre-kit/internal/platform/apierror"
	"sre-kit/internal/projects/domain"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Service struct {
	repo domain.Repository
	now  func() time.Time
}

func NewService(repo domain.Repository) *Service { return &Service{repo: repo, now: time.Now} }

func validate(name, slug string) error {
	if strings.TrimSpace(name) == "" {
		return apierror.Invalid("name is required")
	}
	if !slugPattern.MatchString(slug) {
		return apierror.Invalid("slug must contain lowercase letters, numbers and single hyphens")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, name, slug, description string) (domain.Project, error) {
	if err := validate(name, slug); err != nil {
		return domain.Project{}, err
	}
	now := s.now().UTC()
	project := domain.Project{ID: uuid.NewString(), Name: strings.TrimSpace(name), Slug: slug,
		Description: strings.TrimSpace(description), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, project); err != nil {
		return domain.Project{}, fmt.Errorf("projects: create: %w", err)
	}
	return project, nil
}

func (s *Service) Update(ctx context.Context, id string, name, slug, description *string) (domain.Project, error) {
	project, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Project{}, err
	}
	if name != nil {
		project.Name = strings.TrimSpace(*name)
	}
	if slug != nil {
		project.Slug = *slug
	}
	if description != nil {
		project.Description = strings.TrimSpace(*description)
	}
	if err := validate(project.Name, project.Slug); err != nil {
		return domain.Project{}, err
	}
	project.UpdatedAt = s.now().UTC()
	if err := s.repo.Update(ctx, project); err != nil {
		return domain.Project{}, fmt.Errorf("projects: update: %w", err)
	}
	return project, nil
}

func (s *Service) List(ctx context.Context) ([]domain.Project, error) { return s.repo.List(ctx) }
func (s *Service) Delete(ctx context.Context, id string) error {
	if id == "default" {
		return apierror.Invalid("the default project cannot be deleted")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("projects: delete: %w", err)
	}
	return nil
}
