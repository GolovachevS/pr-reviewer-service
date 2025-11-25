package service

import (
	"context"

	"github.com/GolovachevS/pr-reviewer-service/internal/domain"
)

// CreatePullRequestInput carries payload for PR creation.
type CreatePullRequestInput struct {
	PullRequestID   string
	PullRequestName string
	AuthorID        string
}

// Service orchestrates domain logic.
type Service struct {
	repo   Repository
	picker ReviewerPicker
}

// Repository defines required storage methods to satisfy business flows.
type Repository interface {
	CreateTeam(ctx context.Context, team domain.Team) (domain.Team, error)
	GetTeam(ctx context.Context, teamName string) (domain.Team, error)
	SetUserActive(ctx context.Context, userID string, isActive bool) (domain.User, error)
	CreatePullRequest(ctx context.Context, input CreatePullRequestInput, pick func([]string, int) []string) (domain.PullRequest, error)
	MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error)
	ReassignReviewer(ctx context.Context, prID, oldUserID string, pick func([]string) (string, bool)) (domain.PullRequest, string, error)
	GetUserReviews(ctx context.Context, userID string) (domain.UserReviews, error)
}

// New returns a configured service.
func New(repo Repository, picker ReviewerPicker) *Service {
	if picker == nil {
		picker = NewRandomPicker()
	}
	return &Service{repo: repo, picker: picker}
}

func (s *Service) CreateTeam(ctx context.Context, team domain.Team) (domain.Team, error) {
	return s.repo.CreateTeam(ctx, team)
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	return s.repo.GetTeam(ctx, teamName)
}

func (s *Service) SetUserActive(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	return s.repo.SetUserActive(ctx, userID, isActive)
}

func (s *Service) CreatePullRequest(ctx context.Context, input CreatePullRequestInput) (domain.PullRequest, error) {
	return s.repo.CreatePullRequest(ctx, input, func(ids []string, limit int) []string {
		return s.picker.Pick(ids, limit)
	})
}

func (s *Service) MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	return s.repo.MergePullRequest(ctx, prID)
}

func (s *Service) ReassignReviewer(ctx context.Context, prID, oldUserID string) (domain.PullRequest, string, error) {
	return s.repo.ReassignReviewer(ctx, prID, oldUserID, s.picker.PickOne)
}

func (s *Service) GetUserReviews(ctx context.Context, userID string) (domain.UserReviews, error) {
	return s.repo.GetUserReviews(ctx, userID)
}
