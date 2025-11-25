package domain

import (
	"net/http"
)

type ErrorCode string

const (
	ErrCodeTeamExists  ErrorCode = "TEAM_EXISTS"
	ErrCodePRExists    ErrorCode = "PR_EXISTS"
	ErrCodePRMerged    ErrorCode = "PR_MERGED"
	ErrCodeNotAssigned ErrorCode = "NOT_ASSIGNED"
	ErrCodeNoCandidate ErrorCode = "NO_CANDIDATE"
	ErrCodeNotFound    ErrorCode = "NOT_FOUND"
	ErrCodeInternal    ErrorCode = "INTERNAL"
)

// AppError keeps domain level errors consistent.
type AppError struct {
	Code    ErrorCode
	Message string
	Status  int
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewTeamExistsError(err error) *AppError {
	return &AppError{Code: ErrCodeTeamExists, Message: "team already exists", Status: http.StatusBadRequest, Err: err}
}

func NewNotFoundError(message string, err error) *AppError {
	return &AppError{Code: ErrCodeNotFound, Message: message, Status: http.StatusNotFound, Err: err}
}

func NewPRExistsError(err error) *AppError {
	return &AppError{Code: ErrCodePRExists, Message: "pull request already exists", Status: http.StatusConflict, Err: err}
}

func NewPRMergedError() *AppError {
	return &AppError{Code: ErrCodePRMerged, Message: "cannot mutate merged pull request", Status: http.StatusConflict}
}

func NewNotAssignedError() *AppError {
	return &AppError{Code: ErrCodeNotAssigned, Message: "reviewer is not assigned to this pull request", Status: http.StatusConflict}
}

func NewNoCandidateError() *AppError {
	return &AppError{Code: ErrCodeNoCandidate, Message: "no active replacement candidate in team", Status: http.StatusConflict}
}
