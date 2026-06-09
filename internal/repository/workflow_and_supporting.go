package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"project-management-platform/internal/models"
)

var ErrConflict = errors.New("optimistic locking conflict: resource was modified by another request")

// --- Workflow Repository ---

type WorkflowRepo struct{ db *gorm.DB }

func NewWorkflowRepo(db *gorm.DB) *WorkflowRepo { return &WorkflowRepo{db: db} }

func (r *WorkflowRepo) CreateStatus(ctx context.Context, status *models.WorkflowStatus) error {
	return r.db.WithContext(ctx).Create(status).Error
}

func (r *WorkflowRepo) ListStatuses(ctx context.Context, projectID uuid.UUID) ([]models.WorkflowStatus, error) {
	var statuses []models.WorkflowStatus
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("position").
		Find(&statuses).Error
	return statuses, err
}

func (r *WorkflowRepo) GetStatusByID(ctx context.Context, id uuid.UUID) (*models.WorkflowStatus, error) {
	var status models.WorkflowStatus
	err := r.db.WithContext(ctx).First(&status, "id = ?", id).Error
	return &status, err
}

func (r *WorkflowRepo) GetDefaultStatus(ctx context.Context, projectID uuid.UUID) (*models.WorkflowStatus, error) {
	var status models.WorkflowStatus
	// Default = lowest position number (first column)
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("position ASC").
		First(&status).Error
	return &status, err
}

func (r *WorkflowRepo) CreateTransition(ctx context.Context, t *models.WorkflowTransition) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *WorkflowRepo) ListTransitions(ctx context.Context, projectID uuid.UUID) ([]models.WorkflowTransition, error) {
	var transitions []models.WorkflowTransition
	err := r.db.WithContext(ctx).
		Preload("FromStatus").
		Preload("ToStatus").
		Where("project_id = ?", projectID).
		Find(&transitions).Error
	return transitions, err
}

// GetTransition checks if a specific from->to transition is allowed.
func (r *WorkflowRepo) GetTransition(ctx context.Context, projectID, fromStatusID, toStatusID uuid.UUID) (*models.WorkflowTransition, error) {
	var t models.WorkflowTransition
	err := r.db.WithContext(ctx).
		Preload("FromStatus").
		Preload("ToStatus").
		Where("project_id = ? AND from_status_id = ? AND to_status_id = ?", projectID, fromStatusID, toStatusID).
		First(&t).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &t, err
}

// GetAllowedTransitions returns all statuses reachable from the given status.
func (r *WorkflowRepo) GetAllowedTransitions(ctx context.Context, projectID, fromStatusID uuid.UUID) ([]models.WorkflowTransition, error) {
	var transitions []models.WorkflowTransition
	err := r.db.WithContext(ctx).
		Preload("ToStatus").
		Where("project_id = ? AND from_status_id = ?", projectID, fromStatusID).
		Find(&transitions).Error
	return transitions, err
}

// --- Comment Repository ---

type CommentRepo struct{ db *gorm.DB }

func NewCommentRepo(db *gorm.DB) *CommentRepo { return &CommentRepo{db: db} }

func (r *CommentRepo) Create(ctx context.Context, comment *models.Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *CommentRepo) ListByIssue(ctx context.Context, issueID uuid.UUID) ([]models.Comment, error) {
	var comments []models.Comment
	// Only top-level comments; replies loaded via Preload
	err := r.db.WithContext(ctx).
		Preload("Author").
		Preload("Replies.Author").
		Where("issue_id = ? AND parent_id IS NULL", issueID).
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}

// --- Activity Log Repository ---

type ActivityRepo struct{ db *gorm.DB }

func NewActivityRepo(db *gorm.DB) *ActivityRepo { return &ActivityRepo{db: db} }

func (r *ActivityRepo) Create(ctx context.Context, log *models.ActivityLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *ActivityRepo) ListByProject(ctx context.Context, projectID uuid.UUID, cursor string, limit int) ([]models.ActivityLog, int64, error) {
	var logs []models.ActivityLog
	var total int64

	q := r.db.WithContext(ctx).
		Preload("User").
		Where("project_id = ?", projectID)

	q.Model(&models.ActivityLog{}).Count(&total)

	q = q.Order("created_at DESC").Limit(limit + 1)

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			var ref models.ActivityLog
			if r.db.Select("created_at").First(&ref, "id = ?", cursorID).Error == nil {
				q = q.Where("created_at < ?", ref.CreatedAt)
			}
		}
	}

	err := q.Find(&logs).Error
	return logs, total, err
}

// --- Notification Repository ---

type NotificationRepo struct{ db *gorm.DB }

func NewNotificationRepo(db *gorm.DB) *NotificationRepo { return &NotificationRepo{db: db} }

func (r *NotificationRepo) Create(ctx context.Context, notif *models.Notification) error {
	return r.db.WithContext(ctx).Create(notif).Error
}

func (r *NotificationRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Notification, error) {
	var notifs []models.Notification
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&notifs).Error
	return notifs, err
}

func (r *NotificationRepo) MarkRead(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("id = ?", id).
		Update("read", true).Error
}

// --- Watcher Repository ---

type WatcherRepo struct{ db *gorm.DB }

func NewWatcherRepo(db *gorm.DB) *WatcherRepo { return &WatcherRepo{db: db} }

func (r *WatcherRepo) Watch(ctx context.Context, watcher *models.Watcher) error {
	return r.db.WithContext(ctx).Create(watcher).Error
}

func (r *WatcherRepo) Unwatch(ctx context.Context, issueID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("issue_id = ? AND user_id = ?", issueID, userID).
		Delete(&models.Watcher{}).Error
}

func (r *WatcherRepo) ListByIssue(ctx context.Context, issueID uuid.UUID) ([]models.Watcher, error) {
	var watchers []models.Watcher
	err := r.db.WithContext(ctx).Where("issue_id = ?", issueID).Find(&watchers).Error
	return watchers, err
}

// GetWatcherUserIDs returns all user IDs watching a given issue.
func (r *WatcherRepo) GetWatcherUserIDs(ctx context.Context, issueID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).
		Model(&models.Watcher{}).
		Where("issue_id = ?", issueID).
		Pluck("user_id", &ids).Error
	return ids, err
}

// --- Search Repository ---

type SearchRepo struct{ db *gorm.DB }

func NewSearchRepo(db *gorm.DB) *SearchRepo { return &SearchRepo{db: db} }

// FullTextSearch searches issues using PostgreSQL tsvector.
// Searches across issue titles, descriptions, AND comment bodies.
func (r *SearchRepo) FullTextSearch(ctx context.Context, query string, projectID *uuid.UUID, limit int, cursor string) ([]models.Issue, int64, error) {
	var issues []models.Issue
	var total int64

	// Search issues directly OR issues that have matching comments
	q := r.db.WithContext(ctx).
		Preload("Status").
		Preload("Assignee").
		Where(
			"search_vector @@ plainto_tsquery('english', ?) OR id IN (SELECT issue_id FROM comments WHERE search_vector @@ plainto_tsquery('english', ?))",
			query, query,
		)

	if projectID != nil {
		q = q.Where("project_id = ?", *projectID)
	}

	q.Model(&models.Issue{}).Count(&total)
	q = q.Order("created_at DESC").Limit(limit + 1)

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			var ref models.Issue
			if r.db.Select("created_at").First(&ref, "id = ?", cursorID).Error == nil {
				q = q.Where("created_at < ?", ref.CreatedAt)
			}
		}
	}

	err := q.Find(&issues).Error
	return issues, total, err
}

// StructuredSearch filters issues by structured fields (status, assignee, priority, type, labels).
func (r *SearchRepo) StructuredSearch(ctx context.Context, filters map[string]string, projectID *uuid.UUID, limit int, cursor string) ([]models.Issue, int64, error) {
	var issues []models.Issue
	var total int64

	q := r.db.WithContext(ctx).
		Preload("Status").
		Preload("Assignee")

	if projectID != nil {
		q = q.Where("project_id = ?", *projectID)
	}

	// Apply structured filters
	if status := filters["status"]; status != "" {
		q = q.Joins("JOIN workflow_statuses ON workflow_statuses.id = issues.status_id").
			Where("workflow_statuses.name = ?", status)
	}
	if assignee := filters["assignee"]; assignee != "" {
		q = q.Joins("JOIN users ON users.id = issues.assignee_id").
			Where("users.display_name = ?", assignee)
	}
	if priority := filters["priority"]; priority != "" {
		q = q.Where("issues.priority = ?", priority)
	}
	if issueType := filters["type"]; issueType != "" {
		q = q.Where("issues.type = ?", issueType)
	}
	if label := filters["label"]; label != "" {
		q = q.Where("issues.labels @> ?", `["`+label+`"]`)
	}

	q.Model(&models.Issue{}).Count(&total)
	q = q.Order("issues.created_at DESC").Limit(limit + 1)

	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			var ref models.Issue
			if r.db.Select("created_at").First(&ref, "id = ?", cursorID).Error == nil {
				q = q.Where("issues.created_at < ?", ref.CreatedAt)
			}
		}
	}

	err := q.Find(&issues).Error
	return issues, total, err
}

// --- Custom Field Definition Repository ---

type CustomFieldRepo struct{ db *gorm.DB }

func NewCustomFieldRepo(db *gorm.DB) *CustomFieldRepo { return &CustomFieldRepo{db: db} }

func (r *CustomFieldRepo) Create(ctx context.Context, def *models.CustomFieldDefinition) error {
	return r.db.WithContext(ctx).Create(def).Error
}

func (r *CustomFieldRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.CustomFieldDefinition, error) {
	var defs []models.CustomFieldDefinition
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&defs).Error
	return defs, err
}
