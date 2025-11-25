package transport

import (
	"errors"
	nethttp "net/http"

	"github.com/GolovachevS/pr-reviewer-service/internal/domain"
	"github.com/GolovachevS/pr-reviewer-service/internal/service"
	"github.com/gin-gonic/gin"
)

// NewServer wires routes and returns a configured gin.Engine.
func NewServer(svc *service.Service) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())

	h := handler{svc: svc}

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(nethttp.StatusOK, gin.H{"status": "ok"})
	})

	team := engine.Group("/team")
	{
		team.POST("/add", h.createTeam)
		team.GET("/get", h.getTeam)
	}

	users := engine.Group("/users")
	{
		users.POST("/setIsActive", h.setUserActive)
		users.GET("/getReview", h.getUserReviews)
	}

	pull := engine.Group("/pullRequest")
	{
		pull.POST("/create", h.createPullRequest)
		pull.POST("/merge", h.mergePullRequest)
		pull.POST("/reassign", h.reassignReviewer)
	}

	return engine
}

type handler struct {
	svc *service.Service
}

type createTeamRequest struct {
	TeamName string              `json:"team_name" binding:"required"`
	Members  []domain.TeamMember `json:"members"`
}

type setActiveRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	IsActive *bool  `json:"is_active" binding:"required"`
}

type createPRRequest struct {
	PullRequestID   string `json:"pull_request_id" binding:"required"`
	PullRequestName string `json:"pull_request_name" binding:"required"`
	AuthorID        string `json:"author_id" binding:"required"`
}

type mergePRRequest struct {
	PullRequestID string `json:"pull_request_id" binding:"required"`
}

type reassignRequest struct {
	PullRequestID string `json:"pull_request_id" binding:"required"`
	OldUserID     string `json:"old_user_id" binding:"required"`
}

func (h handler) createTeam(c *gin.Context) {
	var req createTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	if req.Members == nil {
		respondValidationError(c, errors.New("members is required"))
		return
	}

	for _, member := range req.Members {
		if member.UserID == "" || member.Username == "" {
			respondValidationError(c, errMissingMemberFields)
			return
		}
	}

	team, err := h.svc.CreateTeam(c.Request.Context(), domain.Team{
		TeamName: req.TeamName,
		Members:  req.Members,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(nethttp.StatusCreated, gin.H{"team": team})
}

func (h handler) getTeam(c *gin.Context) {
	teamName := c.Query("team_name")
	if teamName == "" {
		respondValidationError(c, errors.New("team_name is required"))
		return
	}

	team, err := h.svc.GetTeam(c.Request.Context(), teamName)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(nethttp.StatusOK, team)
}

func (h handler) setUserActive(c *gin.Context) {
	var req setActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	if req.IsActive == nil {
		respondValidationError(c, errors.New("is_active is required"))
		return
	}
	user, err := h.svc.SetUserActive(c.Request.Context(), req.UserID, *req.IsActive)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"user": user})
}

func (h handler) createPullRequest(c *gin.Context) {
	var req createPRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	pr, err := h.svc.CreatePullRequest(c.Request.Context(), service.CreatePullRequestInput{
		PullRequestID:   req.PullRequestID,
		PullRequestName: req.PullRequestName,
		AuthorID:        req.AuthorID,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(nethttp.StatusCreated, gin.H{"pr": pr})
}

func (h handler) mergePullRequest(c *gin.Context) {
	var req mergePRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	pr, err := h.svc.MergePullRequest(c.Request.Context(), req.PullRequestID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"pr": pr})
}

func (h handler) reassignReviewer(c *gin.Context) {
	var req reassignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	pr, replaced, err := h.svc.ReassignReviewer(c.Request.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"pr": pr, "replaced_by": replaced})
}

func (h handler) getUserReviews(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		respondValidationError(c, errors.New("user_id is required"))
		return
	}
	reviews, err := h.svc.GetUserReviews(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, reviews)
}

var errMissingMemberFields = errors.New("member.user_id and member.username are required")

func respondValidationError(c *gin.Context, err error) {
	writeError(c, nethttp.StatusBadRequest, domain.ErrCodeNotFound, err.Error())
}

func respondError(c *gin.Context, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		writeError(c, appErr.Status, appErr.Code, appErr.Message)
		return
	}
	writeError(c, nethttp.StatusInternalServerError, domain.ErrCodeInternal, "internal error")
}

func writeError(c *gin.Context, status int, code domain.ErrorCode, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
