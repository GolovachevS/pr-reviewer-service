package domain

import "time"

// TeamMember describes a user within a team payload.
type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// Team represents a team with members.
type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

// User is a single user entity.
type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

// PullRequest holds PR data returned to clients.
type PullRequest struct {
	PullRequestID   string     `json:"pull_request_id"`
	PullRequestName string     `json:"pull_request_name"`
	AuthorID        string     `json:"author_id"`
	Status          string     `json:"status"`
	Assigned        []string   `json:"assigned_reviewers"`
	CreatedAt       time.Time  `json:"createdAt"`
	MergedAt        *time.Time `json:"mergedAt"`
}

// PullRequestShort is used for listing assignments per reviewer.
type PullRequestShort struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"`
}

// UserReviews bundles review assignments for response payloads.
type UserReviews struct {
	UserID       string             `json:"user_id"`
	PullRequests []PullRequestShort `json:"pull_requests"`
}
