package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"project-management-platform/internal/middleware"
	"project-management-platform/internal/models"
	"project-management-platform/internal/repository"
	"project-management-platform/internal/services"
)

// --- User Handler ---

type UserHandler struct {
	repo *repository.UserRepo
}

func NewUserHandler(repo *repository.UserRepo) *UserHandler {
	return &UserHandler{repo: repo}
}

type CreateUserRequest struct {
	Email       string `json:"email" binding:"required,email"`
	DisplayName string `json:"display_name" binding:"required"`
	AvatarURL   string `json:"avatar_url"`
}

func (h *UserHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	user := &models.User{Email: req.Email, DisplayName: req.DisplayName, AvatarURL: req.AvatarURL}
	if err := h.repo.Create(c.Request.Context(), user); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusCreated, user, nil)
}

func (h *UserHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid user ID", nil)
		return
	}
	user, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "user not found", nil)
		return
	}
	middleware.Respond(c, http.StatusOK, user, nil)
}

func (h *UserHandler) List(c *gin.Context) {
	users, err := h.repo.List(c.Request.Context())
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, users, nil)
}

// --- Project Handler ---

type ProjectHandler struct {
	svc             *services.ProjectService
	customFieldRepo *repository.CustomFieldRepo
}

func NewProjectHandler(svc *services.ProjectService, cfRepo *repository.CustomFieldRepo) *ProjectHandler {
	return &ProjectHandler{svc: svc, customFieldRepo: cfRepo}
}

type CreateProjectRequest struct {
	Key         string   `json:"key" binding:"required,max=10"`
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	OwnerID     string   `json:"owner_id" binding:"required"`
	Statuses    []string `json:"statuses"` // optional custom workflow statuses
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	ownerID, err := uuid.Parse(req.OwnerID)
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid owner_id", nil)
		return
	}
	project := &models.Project{Key: req.Key, Name: req.Name, Description: req.Description, OwnerID: ownerID}
	if err := h.svc.CreateWithDefaultWorkflow(c.Request.Context(), project, req.Statuses); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusCreated, project, nil)
}

func (h *ProjectHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	project, err := h.svc.ProjectRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "project not found", nil)
		return
	}
	middleware.Respond(c, http.StatusOK, project, nil)
}

func (h *ProjectHandler) List(c *gin.Context) {
	projects, err := h.svc.ProjectRepo.List(c.Request.Context())
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, projects, nil)
}

func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	project, err := h.svc.ProjectRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusNotFound, "NOT_FOUND", "project not found", nil)
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if name, ok := body["name"].(string); ok {
		project.Name = name
	}
	if desc, ok := body["description"].(string); ok {
		project.Description = desc
	}
	if err := h.svc.ProjectRepo.Update(c.Request.Context(), project); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "UPDATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, project, nil)
}

func (h *ProjectHandler) ListStatuses(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	statuses, err := h.svc.WorkflowRepo.ListStatuses(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, statuses, nil)
}

func (h *ProjectHandler) ListTransitions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	transitions, err := h.svc.WorkflowRepo.ListTransitions(c.Request.Context(), id)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, transitions, nil)
}

type CreateTransitionRequest struct {
	FromStatusID     string   `json:"from_status_id" binding:"required"`
	ToStatusID       string   `json:"to_status_id" binding:"required"`
	AutoAssignUserID *string  `json:"auto_assign_user_id"`
	RequiredFields   []string `json:"required_fields"`
}

func (h *ProjectHandler) CreateTransition(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	var req CreateTransitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	fromID, _ := uuid.Parse(req.FromStatusID)
	toID, _ := uuid.Parse(req.ToStatusID)

	t := &models.WorkflowTransition{ProjectID: projectID, FromStatusID: fromID, ToStatusID: toID}
	if req.AutoAssignUserID != nil {
		id, _ := uuid.Parse(*req.AutoAssignUserID)
		t.AutoAssignUserID = &id
	}
	if len(req.RequiredFields) > 0 {
		rfJSON, _ := models.JSONB(mustMarshal(req.RequiredFields)).MarshalJSON()
		t.RequiredFields = models.JSONB(rfJSON)
	}

	if err := h.svc.WorkflowRepo.CreateTransition(c.Request.Context(), t); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusCreated, t, nil)
}

// --- Custom Field Handlers ---

type CreateCustomFieldRequest struct {
	Name      string   `json:"name" binding:"required"`
	FieldType string   `json:"field_type" binding:"required,oneof=text number dropdown date"`
	Options   []string `json:"options"` // for dropdown type
	Required  bool     `json:"required"`
}

func (h *ProjectHandler) CreateCustomField(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	var req CreateCustomFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	def := &models.CustomFieldDefinition{
		ProjectID: projectID,
		Name:      req.Name,
		FieldType: req.FieldType,
		Required:  req.Required,
	}
	if len(req.Options) > 0 {
		def.Options = models.JSONB(mustMarshal(req.Options))
	}
	if err := h.customFieldRepo.Create(c.Request.Context(), def); err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "CREATE_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusCreated, def, nil)
}

func (h *ProjectHandler) ListCustomFields(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid project ID", nil)
		return
	}
	defs, err := h.customFieldRepo.ListByProject(c.Request.Context(), projectID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "LIST_FAILED", err.Error(), nil)
		return
	}
	middleware.Respond(c, http.StatusOK, defs, nil)
}
