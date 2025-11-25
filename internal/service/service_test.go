package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/GolovachevS/pr-reviewer-service/internal/domain"
)

func TestNewUsesDefaultPickerWhenNil(t *testing.T) {
	repo := stubRepository{}
	svc := New(repo, nil)

	if svc.picker == nil {
		t.Fatalf("expected default picker to be injected when nil is provided")
	}

	ids := []string{"a", "b"}
	if picked := svc.picker.Pick(ids, 1); len(ids) > 0 && len(picked) == 0 {
		t.Fatalf("default picker should pick at least one id when available")
	}
}

func TestServiceCreatePullRequestDelegatesPicker(t *testing.T) {
	ctx := context.Background()
	picker := &stubPicker{pickReturn: []string{"u1", "u2"}}

	var receivedIDs []string
	var receivedLimit int
	repo := stubRepository{
		createPullRequestFn: func(_ context.Context, _ CreatePullRequestInput, pick func([]string, int) []string) (domain.PullRequest, error) {
			candidates := []string{"a", "b", "c"}
			receivedLimit = 2
			receivedIDs = pick(candidates, receivedLimit)
			return domain.PullRequest{PullRequestID: "pr-1"}, nil
		},
	}

	svc := New(repo, picker)
	pr, err := svc.CreatePullRequest(ctx, CreatePullRequestInput{PullRequestID: "pr-1"})
	if err != nil {
		t.Fatalf("CreatePullRequest returned error: %v", err)
	}

	if pr.PullRequestID != "pr-1" {
		t.Fatalf("unexpected pull request returned: %+v", pr)
	}

	if !reflect.DeepEqual(picker.lastIDs, []string{"a", "b", "c"}) {
		t.Fatalf("picker received wrong ids: %v", picker.lastIDs)
	}

	if picker.lastLimit != receivedLimit {
		t.Fatalf("picker received limit %d, want %d", picker.lastLimit, receivedLimit)
	}

	if !reflect.DeepEqual(receivedIDs, picker.pickReturn) {
		t.Fatalf("picker result not propagated, got %v want %v", receivedIDs, picker.pickReturn)
	}
}

func TestServiceReassignReviewerUsesPickOne(t *testing.T) {
	ctx := context.Background()
	picker := &stubPicker{pickOneReturn: "new-reviewer", pickOneOK: true}

	var pickInput []string
	repo := stubRepository{
		reassignReviewerFn: func(_ context.Context, prID, oldUserID string, pick func([]string) (string, bool)) (domain.PullRequest, string, error) {
			candidates := []string{"x", "y", "z"}
			pickInput = append([]string(nil), candidates...)
			chosen, _ := pick(candidates)
			return domain.PullRequest{PullRequestID: prID}, chosen, nil
		},
	}

	svc := New(repo, picker)
	pr, replaced, err := svc.ReassignReviewer(ctx, "pr-42", "old")
	if err != nil {
		t.Fatalf("ReassignReviewer returned error: %v", err)
	}

	if pr.PullRequestID != "pr-42" {
		t.Fatalf("unexpected pull request id: %s", pr.PullRequestID)
	}

	if replaced != picker.pickOneReturn {
		t.Fatalf("replaced reviewer mismatch: got %s want %s", replaced, picker.pickOneReturn)
	}

	if !reflect.DeepEqual(pickInput, picker.lastIDs) {
		t.Fatalf("picker PickOne saw %v, want %v", picker.lastIDs, pickInput)
	}
}

type stubPicker struct {
	pickReturn    []string
	lastIDs       []string
	lastLimit     int
	pickOneReturn string
	pickOneOK     bool
}

func (s *stubPicker) Pick(ids []string, limit int) []string {
	s.lastIDs = append([]string(nil), ids...)
	s.lastLimit = limit
	return append([]string(nil), s.pickReturn...)
}

func (s *stubPicker) PickOne(ids []string) (string, bool) {
	s.lastIDs = append([]string(nil), ids...)
	return s.pickOneReturn, s.pickOneOK
}

type stubRepository struct {
	createTeamFn        func(context.Context, domain.Team) (domain.Team, error)
	getTeamFn           func(context.Context, string) (domain.Team, error)
	setUserActiveFn     func(context.Context, string, bool) (domain.User, error)
	createPullRequestFn func(context.Context, CreatePullRequestInput, func([]string, int) []string) (domain.PullRequest, error)
	mergePullRequestFn  func(context.Context, string) (domain.PullRequest, error)
	reassignReviewerFn  func(context.Context, string, string, func([]string) (string, bool)) (domain.PullRequest, string, error)
	getUserReviewsFn    func(context.Context, string) (domain.UserReviews, error)
}

func (s stubRepository) CreateTeam(ctx context.Context, team domain.Team) (domain.Team, error) {
	if s.createTeamFn != nil {
		return s.createTeamFn(ctx, team)
	}
	return domain.Team{}, nil
}

func (s stubRepository) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	if s.getTeamFn != nil {
		return s.getTeamFn(ctx, teamName)
	}
	return domain.Team{}, nil
}

func (s stubRepository) SetUserActive(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	if s.setUserActiveFn != nil {
		return s.setUserActiveFn(ctx, userID, isActive)
	}
	return domain.User{}, nil
}

func (s stubRepository) CreatePullRequest(ctx context.Context, input CreatePullRequestInput, pick func([]string, int) []string) (domain.PullRequest, error) {
	if s.createPullRequestFn != nil {
		return s.createPullRequestFn(ctx, input, pick)
	}
	return domain.PullRequest{}, nil
}

func (s stubRepository) MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	if s.mergePullRequestFn != nil {
		return s.mergePullRequestFn(ctx, prID)
	}
	return domain.PullRequest{}, nil
}

func (s stubRepository) ReassignReviewer(ctx context.Context, prID, oldUserID string, pick func([]string) (string, bool)) (domain.PullRequest, string, error) {
	if s.reassignReviewerFn != nil {
		return s.reassignReviewerFn(ctx, prID, oldUserID, pick)
	}
	return domain.PullRequest{}, "", nil
}

func (s stubRepository) GetUserReviews(ctx context.Context, userID string) (domain.UserReviews, error) {
	if s.getUserReviewsFn != nil {
		return s.getUserReviewsFn(ctx, userID)
	}
	return domain.UserReviews{}, nil
}
