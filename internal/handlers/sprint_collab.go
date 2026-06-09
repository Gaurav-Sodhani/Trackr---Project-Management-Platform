package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"project-management-platform/internal/middleware"
	"project-management-platform/internal/models"
	"project-management-platform/internal/repository"
	"project-management-platform/internal/services"
)

// --- Sprint Handler ---

type SprintHandler struct {
	svc *services.SprintService
}

func NewSprintHandler(svc *services.SprintService) *SprintHandler {
	return &SprintHandler{svc: svc}
}

type CreateSprintRequest struct {
	Name string `json:"name" binding:"required"`
	Goal string `json:"goal"`
}

func (h *SprintHandler) Create(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	var req CreateSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	sprint := &models.Sprint{ProjectID: projectID, Name: req.Name, Goal: req.Goal, Status: models.SprintPlanned}
	if err := h.svc.SprintRepo.Create(c.Request.Context(), sprint); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusCreated, sprint, nil)
}

func (h *SprintHandler) List(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	sprints, err := h.svc.SprintRepo.ListByProject(c.Request.Context(), projectID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, sprints, nil)
}

func (h *SprintHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid sprint ID", nil)
		return
	}
	sprint, err := h.svc.SprintRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "sprint not found", nil)
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if name, ok := body["name"].(string); ok {
		sprint.Name = name
	}
	if goal, ok := body["goal"].(string); ok {
		sprint.Goal = goal
	}
	if err := h.svc.SprintRepo.Update(c.Request.Context(), sprint); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, sprint, nil)
}

type StartSprintRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

func (h *SprintHandler) Start(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid sprint ID", nil)
		return
	}
	var req StartSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	userID, _ := uuid.Parse(req.UserID)
	sprint, err := h.svc.Start(c.Request.Context(), id, userID)
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "START_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, sprint, nil)
}

type CompleteSprintRequest struct {
	CarryOverIssueIDs []string `json:"carry_over_issue_ids"`
	NewSprintID       *string  `json:"new_sprint_id"`
	UserID            string   `json:"user_id" binding:"required"`
}

func (h *SprintHandler) Complete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid sprint ID", nil)
		return
	}
	var req CompleteSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	carryIDs := make([]uuid.UUID, len(req.CarryOverIssueIDs))
	for i, sid := range req.CarryOverIssueIDs {
		carryIDs[i], _ = uuid.Parse(sid)
	}
	var newSprintID *uuid.UUID
	if req.NewSprintID != nil {
		nsid, _ := uuid.Parse(*req.NewSprintID)
		newSprintID = &nsid
	}
	userID, _ := uuid.Parse(req.UserID)
	result, err := h.svc.Complete(c.Request.Context(), id, carryIDs, newSprintID, userID)
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "COMPLETE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, result, nil)
}

// --- Comment Handler ---

type CommentHandler struct {
	svc *services.CommentService
}

func NewCommentHandler(svc *services.CommentService) *CommentHandler {
	return &CommentHandler{svc: svc}
}

type CreateCommentRequest struct {
	AuthorID string  `json:"author_id" binding:"required"`
	ParentID *string `json:"parent_id"`
	Body     string  `json:"body" binding:"required"`
}

func (h *CommentHandler) Create(c *gin.Context) {
	issueID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	authorID, _ := uuid.Parse(req.AuthorID)
	comment := &models.Comment{IssueID: issueID, AuthorID: authorID, Body: req.Body}
	if req.ParentID != nil {
		pid, _ := uuid.Parse(*req.ParentID)
		comment.ParentID = &pid
	}
	if err := h.svc.Create(c.Request.Context(), comment); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusCreated, comment, nil)
}

func (h *CommentHandler) List(c *gin.Context) {
	issueID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}
	comments, err := h.svc.CommentRepo.ListByIssue(c.Request.Context(), issueID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, comments, nil)
}

// --- Activity Handler ---

type ActivityHandler struct {
	repo *repository.ActivityRepo
}

func NewActivityHandler(repo *repository.ActivityRepo) *ActivityHandler {
	return &ActivityHandler{repo: repo}
}

func (h *ActivityHandler) List(c *gin.Context) {
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
	logs, total, err := h.repo.ListByProject(c.Request.Context(), projectID, cursor, limit)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	hasMore := len(logs) > limit
	if hasMore {
		logs = logs[:limit]
	}
	nextCursor := ""
	if hasMore && len(logs) > 0 {
		nextCursor = logs[len(logs)-1].ID.String()
	}
	middleware.Respond(c, http.StatusOK, logs, &middleware.PaginationMeta{NextCursor: nextCursor, HasMore: hasMore, Total: total})
}

// --- Search Handler ---

type SearchHandler struct {
	svc *services.SearchService
}

func NewSearchHandler(svc *services.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) Search(c *gin.Context) {
	q := c.Query("q")
	cursor := c.Query("cursor")
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	var projectID *uuid.UUID
	if pid := c.Query("project_id"); pid != "" {
		id, err := uuid.Parse(pid)
		if err == nil {
			projectID = &id
		}
	}

	if q != "" {
		issues, total, err := h.svc.Repo.FullTextSearch(c.Request.Context(), q, projectID, limit, cursor)
		if err != nil {
			middleware.RespondError(c, http.StatusInternalServerError, "SEARCH_FAILED", err.Error(), nil)
			return
		}
		hasMore := len(issues) > limit
		if hasMore {
			issues = issues[:limit]
		}
		nextCursor := ""
		if hasMore && len(issues) > 0 {
			nextCursor = issues[len(issues)-1].ID.String()
		}
		middleware.Respond(c, http.StatusOK, issues, &middleware.PaginationMeta{NextCursor: nextCursor, HasMore: hasMore, Total: total})
		return
	}

	// Structured query filters
	filters := map[string]string{
		"status": c.Query("status"), "assignee": c.Query("assignee"),
		"priority": c.Query("priority"), "type": c.Query("type"), "label": c.Query("label"),
	}
	issues, total, err := h.svc.Repo.StructuredSearch(c.Request.Context(), filters, projectID, limit, cursor)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "SEARCH_FAILED", err.Error(), nil)
		return
	}
	hasMore := len(issues) > limit
	if hasMore {
		issues = issues[:limit]
	}
	nextCursor := ""
	if hasMore && len(issues) > 0 {
		nextCursor = issues[len(issues)-1].ID.String()
	}
	middleware.Respond(c, http.StatusOK, issues, &middleware.PaginationMeta{NextCursor: nextCursor, HasMore: hasMore, Total: total})
}

// --- Notification Handler ---

type NotificationHandler struct {
	repo *repository.NotificationRepo
}

func NewNotificationHandler(repo *repository.NotificationRepo) *NotificationHandler {
	return &NotificationHandler{repo: repo}
}

func (h *NotificationHandler) List(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid user ID", nil)
		return
	}
	notifs, err := h.repo.ListByUser(c.Request.Context(), userID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, notifs, nil)
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid notification ID", nil)
		return
	}
	if err := h.repo.MarkRead(c.Request.Context(), id); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, map[string]string{"message": "marked as read"}, nil)
}

// --- Watcher Handler ---

type WatcherHandler struct {
	repo *repository.WatcherRepo
}

func NewWatcherHandler(repo *repository.WatcherRepo) *WatcherHandler {
	return &WatcherHandler{repo: repo}
}

type WatchRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

func (h *WatcherHandler) Watch(c *gin.Context) {
	issueID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}
	var req WatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	userID, _ := uuid.Parse(req.UserID)
	watcher := &models.Watcher{IssueID: issueID, UserID: userID}
	if err := h.repo.Watch(c.Request.Context(), watcher); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "WATCH_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusCreated, watcher, nil)
}

func (h *WatcherHandler) Unwatch(c *gin.Context) {
	issueID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}
	var req WatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	userID, _ := uuid.Parse(req.UserID)
	if err := h.repo.Unwatch(c.Request.Context(), issueID, userID); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "UNWATCH_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, map[string]string{"message": "unwatched"}, nil)
}

func (h *WatcherHandler) List(c *gin.Context) {
	issueID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid issue ID", nil)
		return
	}
	watchers, err := h.repo.ListByIssue(c.Request.Context(), issueID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, watchers, nil)
}
