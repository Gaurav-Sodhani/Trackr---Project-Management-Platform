package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"project-management-platform/internal/models"
)

// --- User Repository ---

type UserRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	return &user, err
}

func (r *UserRepo) List(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).Find(&users).Error
	return users, err
}

// --- Project Repository ---

type ProjectRepo struct{ db *gorm.DB }

func NewProjectRepo(db *gorm.DB) *ProjectRepo { return &ProjectRepo{db: db} }

func (r *ProjectRepo) Create(ctx context.Context, project *models.Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *ProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	var project models.Project
	err := r.db.WithContext(ctx).Preload("Owner").First(&project, "id = ?", id).Error
	return &project, err
}

func (r *ProjectRepo) List(ctx context.Context) ([]models.Project, error) {
	var projects []models.Project
	err := r.db.WithContext(ctx).Preload("Owner").Find(&projects).Error
	return projects, err
}

func (r *ProjectRepo) Update(ctx context.Context, project *models.Project) error {
	return r.db.WithContext(ctx).Save(project).Error
}

// IncrementIssueCount atomically increments and returns the next issue number.
func (r *ProjectRepo) IncrementIssueCount(ctx context.Context, projectID uuid.UUID) (int, error) {
	var project models.Project
	err := r.db.WithContext(ctx).
		Model(&project).
		Where("id = ?", projectID).
		Update("issue_count", gorm.Expr("issue_count + 1")).Error
	if err != nil {
		return 0, err
	}
	err = r.db.WithContext(ctx).Select("issue_count").First(&project, "id = ?", projectID).Error
	return project.IssueCount, err
}

// --- Issue Repository ---

type IssueRepo struct{ db *gorm.DB }

func NewIssueRepo(db *gorm.DB) *IssueRepo { return &IssueRepo{db: db} }

func (r *IssueRepo) Create(ctx context.Context, issue *models.Issue) error {
	return r.db.WithContext(ctx).Create(issue).Error
}

func (r *IssueRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Issue, error) {
	var issue models.Issue
	err := r.db.WithContext(ctx).
		Preload("Status").
		Preload("Assignee").
		Preload("Reporter").
		Preload("Children").
		First(&issue, "id = ?", id).Error
	return &issue, err
}

func (r *IssueRepo) ListByProject(ctx context.Context, projectID uuid.UUID, cursor string, limit int) ([]models.Issue, error) {
	var issues []models.Issue
	q := r.db.WithContext(ctx).
		Preload("Status").
		Preload("Assignee").
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Limit(limit + 1) // fetch one extra to check "has_more"

	if cursor != "" {
		// Cursor is the ID of the last item seen
		cursorID, err := uuid.Parse(cursor)
		if err == nil {
			var ref models.Issue
			if r.db.Select("created_at").First(&ref, "id = ?", cursorID).Error == nil {
				q = q.Where("created_at < ? OR (created_at = ? AND id < ?)", ref.CreatedAt, ref.CreatedAt, cursorID)
			}
		}
	}
	err := q.Find(&issues).Error
	return issues, err
}

// ListBySprintID returns all issues in a given sprint.
func (r *IssueRepo) ListBySprintID(ctx context.Context, sprintID uuid.UUID) ([]models.Issue, error) {
	var issues []models.Issue
	err := r.db.WithContext(ctx).
		Preload("Status").
		Where("sprint_id = ?", sprintID).
		Find(&issues).Error
	return issues, err
}

// ListBacklog returns issues with no sprint assigned.
func (r *IssueRepo) ListBacklog(ctx context.Context, projectID uuid.UUID) ([]models.Issue, error) {
	var issues []models.Issue
	err := r.db.WithContext(ctx).
		Preload("Status").
		Where("project_id = ? AND sprint_id IS NULL", projectID).
		Find(&issues).Error
	return issues, err
}

// UpdateWithVersion uses optimistic locking -- only updates if version matches.
func (r *IssueRepo) UpdateWithVersion(ctx context.Context, issue *models.Issue) error {
	result := r.db.WithContext(ctx).
		Model(issue).
		Where("id = ? AND version = ?", issue.ID, issue.Version).
		Updates(map[string]interface{}{
			"title":         issue.Title,
			"description":   issue.Description,
			"status_id":     issue.StatusID,
			"priority":      issue.Priority,
			"assignee_id":   issue.AssigneeID,
			"sprint_id":     issue.SprintID,
			"story_points":  issue.StoryPoints,
			"labels":        issue.Labels,
			"custom_fields": issue.CustomFields,
			"version":       gorm.Expr("version + 1"),
		})
	if result.RowsAffected == 0 {
		return ErrConflict
	}
	return result.Error
}

func (r *IssueRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Issue{}, "id = ?", id).Error
}

// GetBoardByProject returns all issues grouped by status for board view.
func (r *IssueRepo) GetBoardByProject(ctx context.Context, projectID uuid.UUID) ([]models.Issue, error) {
	var issues []models.Issue
	err := r.db.WithContext(ctx).
		Preload("Status").
		Preload("Assignee").
		Where("project_id = ?", projectID).
		Order("status_id, created_at").
		Find(&issues).Error
	return issues, err
}

// BulkUpdateSprint moves multiple issues to a new sprint (for carry-over).
func (r *IssueRepo) BulkUpdateSprint(ctx context.Context, issueIDs []uuid.UUID, sprintID *uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Issue{}).
		Where("id IN ?", issueIDs).
		Update("sprint_id", sprintID).Error
}

// --- Sprint Repository ---

type SprintRepo struct{ db *gorm.DB }

func NewSprintRepo(db *gorm.DB) *SprintRepo { return &SprintRepo{db: db} }

func (r *SprintRepo) Create(ctx context.Context, sprint *models.Sprint) error {
	return r.db.WithContext(ctx).Create(sprint).Error
}

func (r *SprintRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Sprint, error) {
	var sprint models.Sprint
	err := r.db.WithContext(ctx).First(&sprint, "id = ?", id).Error
	return &sprint, err
}

func (r *SprintRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.Sprint, error) {
	var sprints []models.Sprint
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&sprints).Error
	return sprints, err
}

func (r *SprintRepo) Update(ctx context.Context, sprint *models.Sprint) error {
	return r.db.WithContext(ctx).Save(sprint).Error
}

// GetActiveSprint returns the currently active sprint for a project (if any).
func (r *SprintRepo) GetActiveSprint(ctx context.Context, projectID uuid.UUID) (*models.Sprint, error) {
	var sprint models.Sprint
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND status = ?", projectID, models.SprintActive).
		First(&sprint).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &sprint, err
}
