package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"project-management-platform/internal/middleware"
	"project-management-platform/internal/models"
	"project-management-platform/internal/repository"
	"project-management-platform/internal/services"
)

// --- Issue Handler ---

type IssueHandler struct {
	svc *services.IssueService
}

func NewIssueHandler(svc *services.IssueService) *IssueHandler {
	return &IssueHandler{svc: svc}
}

type CreateIssueRequest struct {
	Type        string   `json:"type" binding:"required,oneof=epic story task bug subtask"`
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description"`
	Priority    string   `json:"priority" binding:"omitempty,oneof=low medium high critical"`
	AssigneeID  *string  `json:"assignee_id"`
	ReporterID  string   `json:"reporter_id" binding:"required"`
	SprintID    *string  `json:"sprint_id"`
	ParentID    *string  `json:"parent_id"`
	StoryPoints *int     `json:"story_points"`
	Labels      []string `json:"labels"`
}

func (h *IssueHandler) Create(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}

	var req CreateIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	reporterID, _ := uuid.Parse(req.ReporterID)
	issue := &models.Issue{
		ProjectID:   projectID,
		Type:        req.Type,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		ReporterID:  reporterID,
	}
	if issue.Priority == "" {
		issue.Priority = models.PriorityMedium
	}
	if req.AssigneeID != nil {
		id, _ := uuid.Parse(*req.AssigneeID)
		issue.AssigneeID = &id
	}
	if req.SprintID != nil {
		id, _ := uuid.Parse(*req.SprintID)
		issue.SprintID = &id
	}
	if req.ParentID != nil {
		id, _ := uuid.Parse(*req.ParentID)
		issue.ParentID = &id
	}
	issue.StoryPoints = req.StoryPoints
	if len(req.Labels) > 0 {
		issue.Labels = models.JSONB(mustMarshal(req.Labels))
	}

	if err := h.svc.Create(c.Request.Context(), issue); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusCreated, issue, nil)
}

func (h *IssueHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}
	issue, err := h.svc.IssueRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "issue not found", nil)
		return
	}
	middleware.Respond(c, http.StatusOK, issue, nil)
}

func (h *IssueHandler) List(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}

	cursor := c.Query("cursor")
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	issues, err := h.svc.IssueRepo.ListByProject(c.Request.Context(), projectID, cursor, limit)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}

	// Cursor-based pagination: check if there are more results
	hasMore := len(issues) > limit
	if hasMore {
		issues = issues[:limit]
	}
	nextCursor := ""
	if hasMore && len(issues) > 0 {
		nextCursor = issues[len(issues)-1].ID.String()
	}

	middleware.Respond(c, http.StatusOK, issues, &middleware.PaginationMeta{
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}

func (h *IssueHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}

	issue, err := h.svc.IssueRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "issue not found", nil)
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	// Check optimistic locking version
	if v, ok := body["version"].(float64); ok {
		if int(v) != issue.Version {
			middleware.RespondError(c, http.StatusConflict, "CONFLICT",
				"issue has been modified by another user",
				map[string]int{"current_version": issue.Version, "your_version": int(v)})
			return
		}
	}

	// Apply partial updates
	if title, ok := body["title"].(string); ok {
		issue.Title = title
	}
	if desc, ok := body["description"].(string); ok {
		issue.Description = desc
	}
	if priority, ok := body["priority"].(string); ok {
		issue.Priority = priority
	}
	if sp, ok := body["story_points"].(float64); ok {
		pts := int(sp)
		issue.StoryPoints = &pts
	}
	if assignee, ok := body["assignee_id"].(string); ok {
		aid, _ := uuid.Parse(assignee)
		issue.AssigneeID = &aid
	}
	if labels, ok := body["labels"].([]interface{}); ok {
		issue.Labels = models.JSONB(mustMarshal(labels))
	}
	if cf, ok := body["custom_fields"]; ok {
		issue.CustomFields = models.JSONB(mustMarshal(cf))
	}

	// Extract user ID from request (simplified -- in production this comes from auth)
	userID := issue.ReporterID
	if uid, ok := body["user_id"].(string); ok {
		userID, _ = uuid.Parse(uid)
	}

	if err := h.svc.Update(c.Request.Context(), issue, userID); err != nil {
		if err == repository.ErrConflict {
			middleware.RespondError(c, http.StatusConflict, "CONFLICT", "issue was modified concurrently", nil)
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, issue, nil)
}

func (h *IssueHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}
	if err := h.svc.IssueRepo.Delete(c.Request.Context(), id); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, map[string]string{"message": "issue deleted"}, nil)
}

// Transition handles POST /api/issues/:id/transitions
// This is the core workflow engine endpoint.
type TransitionRequest struct {
	ToStatusID string `json:"to_status_id" binding:"required"`
	UserID     string `json:"user_id" binding:"required"`
}

func (h *IssueHandler) Transition(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}

	var req TransitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	issue, err := h.svc.IssueRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "issue not found", nil)
		return
	}

	toStatusID, _ := uuid.Parse(req.ToStatusID)
	userID, _ := uuid.Parse(req.UserID)

	if err := h.svc.Transition(c.Request.Context(), issue, toStatusID, userID); err != nil {
		// Check if it's a workflow violation (Scenario 3)
		if tErr, ok := err.(*services.TransitionError); ok {
			middleware.RespondError(c, http.StatusUnprocessableEntity, "INVALID_TRANSITION", tErr.Error(), tErr)
			return
		}
		// Check if it's a validation error (required fields missing)
		if vErr, ok := err.(*services.ValidationError); ok {
			middleware.RespondError(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", vErr.Error(), vErr)
			return
		}
		middleware.RespondError(c, http.StatusInternalServerError, "TRANSITION_FAILED", err.Error(), nil)
		return
	}

	// Reload to get updated status details
	issue, _ = h.svc.IssueRepo.GetByID(c.Request.Context(), id)
	middleware.Respond(c, http.StatusOK, issue, nil)
}

// MoveToSprint handles POST /api/issues/:id/move-to-sprint
type MoveToSprintRequest struct {
	SprintID *string `json:"sprint_id"` // nil = move to backlog
	UserID   string  `json:"user_id" binding:"required"`
}

func (h *IssueHandler) MoveToSprint(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}

	var req MoveToSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	var sprintID *uuid.UUID
	if req.SprintID != nil {
		sid, _ := uuid.Parse(*req.SprintID)
		sprintID = &sid
	}
	userID, _ := uuid.Parse(req.UserID)

	if err := h.svc.MoveToSprint(c.Request.Context(), id, sprintID, userID); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "MOVE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, map[string]string{"message": "issue moved"}, nil)
}

// GetBoard returns issues grouped by workflow status (board view).
func (h *IssueHandler) GetBoard(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}

	issues, err := h.svc.IssueRepo.GetBoardByProject(c.Request.Context(), projectID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "BOARD_FAILED", err.Error(), nil)
		return
	}

	// Group issues by status name for board columns
	board := make(map[string][]models.Issue)
	for _, issue := range issues {
		statusName := issue.Status.Name
		board[statusName] = append(board[statusName], issue)
	}

	middleware.Respond(c, http.StatusOK, board, nil)
}

// --- helpers ---

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
