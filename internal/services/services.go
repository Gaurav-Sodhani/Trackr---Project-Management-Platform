package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"

	"project-management-platform/internal/models"
	"project-management-platform/internal/repository"
	"project-management-platform/internal/websocket"
)

// --- Activity Service ---

type ActivityService struct {
	Repo  *repository.ActivityRepo
	WsHub *websocket.Hub
}

func NewActivityService(repo *repository.ActivityRepo, wsHub *websocket.Hub) *ActivityService {
	return &ActivityService{Repo: repo, WsHub: wsHub}
}

func (s *ActivityService) Log(ctx context.Context, projectID uuid.UUID, issueID *uuid.UUID, userID uuid.UUID, action, field, oldVal, newVal string) {
	entry := &models.ActivityLog{
		ProjectID: projectID,
		IssueID:   issueID,
		UserID:    userID,
		Action:    action,
		Field:     field,
		OldValue:  oldVal,
		NewValue:  newVal,
		CreatedAt: time.Now(),
	}
	_ = s.Repo.Create(ctx, entry)
}

// --- Notification Service ---

type NotificationService struct {
	Repo *repository.NotificationRepo
}

func NewNotificationService(repo *repository.NotificationRepo) *NotificationService {
	return &NotificationService{Repo: repo}
}

func (s *NotificationService) Notify(ctx context.Context, userID uuid.UUID, notifType, title, message string, issueID *uuid.UUID) {
	notif := &models.Notification{
		UserID:    userID,
		Type:      notifType,
		Title:     title,
		Message:   message,
		IssueID:   issueID,
		CreatedAt: time.Now(),
	}
	_ = s.Repo.Create(ctx, notif)
}

func (s *NotificationService) NotifyWatchers(ctx context.Context, watcherRepo *repository.WatcherRepo, issueID uuid.UUID, excludeUserID uuid.UUID, notifType, title, message string) {
	userIDs, err := watcherRepo.GetWatcherUserIDs(ctx, issueID)
	if err != nil {
		return
	}
	for _, uid := range userIDs {
		if uid == excludeUserID {
			continue
		}
		s.Notify(ctx, uid, notifType, title, message, &issueID)
	}
}

// --- Workflow Service ---

type WorkflowService struct {
	Repo        *repository.WorkflowRepo
	ActivitySvc *ActivityService
	NotifSvc    *NotificationService
	WatcherRepo *repository.WatcherRepo
	WsHub       *websocket.Hub
}

func NewWorkflowService(repo *repository.WorkflowRepo, activitySvc *ActivityService, notifSvc *NotificationService, watcherRepo *repository.WatcherRepo, wsHub *websocket.Hub) *WorkflowService {
	return &WorkflowService{Repo: repo, ActivitySvc: activitySvc, NotifSvc: notifSvc, WatcherRepo: watcherRepo, WsHub: wsHub}
}

type TransitionError struct {
	CurrentStatus      string   `json:"current_status"`
	RequestedStatus    string   `json:"requested_status"`
	AllowedTransitions []string `json:"allowed_transitions"`
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("cannot transition from '%s' to '%s'", e.CurrentStatus, e.RequestedStatus)
}

type ValidationError struct {
	MissingFields []string `json:"missing_fields"`
	Message       string   `json:"message"`
}

func (e *ValidationError) Error() string { return e.Message }

// ValidateAndTransition checks transition rules, validates required fields, runs auto-actions.
func (s *WorkflowService) ValidateAndTransition(ctx context.Context, issue *models.Issue, toStatusID uuid.UUID, userID uuid.UUID) (*uuid.UUID, *uuid.UUID, error) {
	transition, err := s.Repo.GetTransition(ctx, issue.ProjectID, issue.StatusID, toStatusID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check transition: %w", err)
	}

	if transition == nil {
		// Build error with allowed transitions list
		allowed, _ := s.Repo.GetAllowedTransitions(ctx, issue.ProjectID, issue.StatusID)
		currentStatus, _ := s.Repo.GetStatusByID(ctx, issue.StatusID)
		targetStatus, _ := s.Repo.GetStatusByID(ctx, toStatusID)

		allowedNames := make([]string, len(allowed))
		for i, t := range allowed {
			allowedNames[i] = t.ToStatus.Name
		}
		currentName, targetName := "unknown", "unknown"
		if currentStatus != nil {
			currentName = currentStatus.Name
		}
		if targetStatus != nil {
			targetName = targetStatus.Name
		}
		return nil, nil, &TransitionError{
			CurrentStatus: currentName, RequestedStatus: targetName, AllowedTransitions: allowedNames,
		}
	}

	// Validate required fields
	if len(transition.RequiredFields) > 0 && string(transition.RequiredFields) != "[]" {
		var requiredFields []string
		if err := json.Unmarshal(transition.RequiredFields, &requiredFields); err == nil {
			missing := checkRequiredFields(issue, requiredFields)
			if len(missing) > 0 {
				return nil, nil, &ValidationError{
					MissingFields: missing,
					Message:       fmt.Sprintf("cannot transition: missing required fields: %v", missing),
				}
			}
		}
	}

	// Log activity
	fromStatus, _ := s.Repo.GetStatusByID(ctx, issue.StatusID)
	toStatus, _ := s.Repo.GetStatusByID(ctx, toStatusID)
	fromName, toName := "", ""
	if fromStatus != nil {
		fromName = fromStatus.Name
	}
	if toStatus != nil {
		toName = toStatus.Name
	}
	s.ActivitySvc.Log(ctx, issue.ProjectID, &issue.ID, userID, "transitioned", "status", fromName, toName)

	// Notify watchers
	s.NotifSvc.NotifyWatchers(ctx, s.WatcherRepo, issue.ID, userID,
		"status_change",
		fmt.Sprintf("Issue %s moved to %s", issue.IssueKey, toName),
		fmt.Sprintf("Status changed from %s to %s", fromName, toName),
	)

	// Broadcast WebSocket event
	s.WsHub.PublishEvent("issue_moved", issue.ProjectID, map[string]interface{}{
		"issue_id": issue.ID, "issue_key": issue.IssueKey,
		"from_status": fromName, "to_status": toName,
	})

	return &toStatusID, transition.AutoAssignUserID, nil
}

func checkRequiredFields(issue *models.Issue, required []string) []string {
	var missing []string
	for _, f := range required {
		switch f {
		case "assignee":
			if issue.AssigneeID == nil {
				missing = append(missing, "assignee")
			}
		case "description":
			if issue.Description == "" {
				missing = append(missing, "description")
			}
		case "story_points":
			if issue.StoryPoints == nil {
				missing = append(missing, "story_points")
			}
		}
	}
	return missing
}

// --- Project Service ---

type ProjectService struct {
	ProjectRepo  *repository.ProjectRepo
	WorkflowRepo *repository.WorkflowRepo
}

func NewProjectService(projectRepo *repository.ProjectRepo, workflowRepo *repository.WorkflowRepo) *ProjectService {
	return &ProjectService{ProjectRepo: projectRepo, WorkflowRepo: workflowRepo}
}

func (s *ProjectService) CreateWithDefaultWorkflow(ctx context.Context, project *models.Project, customStatuses []string) error {
	if err := s.ProjectRepo.Create(ctx, project); err != nil {
		return err
	}

	statuses := customStatuses
	if len(statuses) == 0 {
		statuses = []string{"To Do", "In Progress", "In Review", "Done"}
	}

	statusModels := make([]models.WorkflowStatus, len(statuses))
	for i, name := range statuses {
		statusModels[i] = models.WorkflowStatus{ProjectID: project.ID, Name: name, Position: i}
		statusModels[i].ID = uuid.New()
		if err := s.WorkflowRepo.CreateStatus(ctx, &statusModels[i]); err != nil {
			return fmt.Errorf("failed to create status '%s': %w", name, err)
		}
	}

	// Default linear transitions + backward transitions
	for i := 0; i < len(statusModels)-1; i++ {
		t := &models.WorkflowTransition{ProjectID: project.ID, FromStatusID: statusModels[i].ID, ToStatusID: statusModels[i+1].ID}
		if err := s.WorkflowRepo.CreateTransition(ctx, t); err != nil {
			return fmt.Errorf("failed to create transition: %w", err)
		}
		if i > 0 {
			back := &models.WorkflowTransition{ProjectID: project.ID, FromStatusID: statusModels[i].ID, ToStatusID: statusModels[i-1].ID}
			_ = s.WorkflowRepo.CreateTransition(ctx, back)
		}
	}
	return nil
}

// --- Issue Service ---

type IssueService struct {
	IssueRepo   *repository.IssueRepo
	ProjectRepo *repository.ProjectRepo
	WorkflowSvc *WorkflowService
	ActivitySvc *ActivityService
	NotifSvc    *NotificationService
	WatcherRepo *repository.WatcherRepo
	WsHub       *websocket.Hub
}

func NewIssueService(issueRepo *repository.IssueRepo, projectRepo *repository.ProjectRepo, workflowSvc *WorkflowService, activitySvc *ActivityService, notifSvc *NotificationService, watcherRepo *repository.WatcherRepo, wsHub *websocket.Hub) *IssueService {
	return &IssueService{
		IssueRepo: issueRepo, ProjectRepo: projectRepo, WorkflowSvc: workflowSvc,
		ActivitySvc: activitySvc, NotifSvc: notifSvc, WatcherRepo: watcherRepo, WsHub: wsHub,
	}
}

func (s *IssueService) Create(ctx context.Context, issue *models.Issue) error {
	project, err := s.ProjectRepo.GetByID(ctx, issue.ProjectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	num, err := s.ProjectRepo.IncrementIssueCount(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("failed to generate issue key: %w", err)
	}
	issue.IssueKey = fmt.Sprintf("%s-%d", project.Key, num)

	if issue.StatusID == uuid.Nil {
		defaultStatus, err := s.WorkflowSvc.Repo.GetDefaultStatus(ctx, issue.ProjectID)
		if err != nil {
			return fmt.Errorf("failed to get default status: %w", err)
		}
		issue.StatusID = defaultStatus.ID
	}

	if err := s.IssueRepo.Create(ctx, issue); err != nil {
		return err
	}

	_ = s.WatcherRepo.Watch(ctx, &models.Watcher{IssueID: issue.ID, UserID: issue.ReporterID})
	s.ActivitySvc.Log(ctx, issue.ProjectID, &issue.ID, issue.ReporterID, "created", "", "", issue.IssueKey)

	if issue.AssigneeID != nil {
		s.NotifSvc.Notify(ctx, *issue.AssigneeID, "assignment",
			fmt.Sprintf("You were assigned %s", issue.IssueKey), issue.Title, &issue.ID)
	}

	s.WsHub.PublishEvent("issue_created", issue.ProjectID, issue)
	return nil
}

func (s *IssueService) Update(ctx context.Context, issue *models.Issue, userID uuid.UUID) error {
	if err := s.IssueRepo.UpdateWithVersion(ctx, issue); err != nil {
		return err
	}
	s.ActivitySvc.Log(ctx, issue.ProjectID, &issue.ID, userID, "updated", "", "", "")
	s.WsHub.PublishEvent("issue_updated", issue.ProjectID, issue)
	return nil
}

func (s *IssueService) Transition(ctx context.Context, issue *models.Issue, toStatusID uuid.UUID, userID uuid.UUID) error {
	newStatusID, autoAssignUserID, err := s.WorkflowSvc.ValidateAndTransition(ctx, issue, toStatusID, userID)
	if err != nil {
		return err
	}
	issue.StatusID = *newStatusID
	if autoAssignUserID != nil {
		issue.AssigneeID = autoAssignUserID
		s.NotifSvc.Notify(ctx, *autoAssignUserID, "assignment",
			fmt.Sprintf("Auto-assigned to %s", issue.IssueKey),
			"Automatically assigned via workflow transition", &issue.ID)
	}
	return s.IssueRepo.UpdateWithVersion(ctx, issue)
}

func (s *IssueService) MoveToSprint(ctx context.Context, issueID uuid.UUID, sprintID *uuid.UUID, userID uuid.UUID) error {
	issue, err := s.IssueRepo.GetByID(ctx, issueID)
	if err != nil {
		return err
	}
	oldSprintID := issue.SprintID
	issue.SprintID = sprintID
	if err := s.IssueRepo.UpdateWithVersion(ctx, issue); err != nil {
		return err
	}
	oldVal, newVal := "backlog", "backlog"
	if oldSprintID != nil {
		oldVal = oldSprintID.String()
	}
	if sprintID != nil {
		newVal = sprintID.String()
	}
	s.ActivitySvc.Log(ctx, issue.ProjectID, &issue.ID, userID, "sprint_moved", "sprint_id", oldVal, newVal)
	return nil
}

// --- Comment Service ---

type CommentService struct {
	CommentRepo *repository.CommentRepo
	IssueRepo   *repository.IssueRepo
	ActivitySvc *ActivityService
	NotifSvc    *NotificationService
	WatcherRepo *repository.WatcherRepo
	WsHub       *websocket.Hub
}

func NewCommentService(commentRepo *repository.CommentRepo, issueRepo *repository.IssueRepo, activitySvc *ActivityService, notifSvc *NotificationService, watcherRepo *repository.WatcherRepo, wsHub *websocket.Hub) *CommentService {
	return &CommentService{
		CommentRepo: commentRepo, IssueRepo: issueRepo, ActivitySvc: activitySvc,
		NotifSvc: notifSvc, WatcherRepo: watcherRepo, WsHub: wsHub,
	}
}

var mentionRegex = regexp.MustCompile(`@(\w+)`)

func (s *CommentService) Create(ctx context.Context, comment *models.Comment) error {
	// Load the issue for project context
	issue, err := s.IssueRepo.GetByID(ctx, comment.IssueID)
	if err != nil {
		return fmt.Errorf("issue not found: %w", err)
	}

	// Extract @mentions
	matches := mentionRegex.FindAllStringSubmatch(comment.Body, -1)
	if len(matches) > 0 {
		mentioned := make([]string, len(matches))
		for i, m := range matches {
			mentioned[i] = m[1]
		}
		mentionJSON, _ := json.Marshal(mentioned)
		comment.Mentions = models.JSONB(mentionJSON)
	}

	if err := s.CommentRepo.Create(ctx, comment); err != nil {
		return err
	}

	_ = s.WatcherRepo.Watch(ctx, &models.Watcher{IssueID: comment.IssueID, UserID: comment.AuthorID})

	bodyPreview := comment.Body
	if len(bodyPreview) > 100 {
		bodyPreview = bodyPreview[:100]
	}
	s.ActivitySvc.Log(ctx, issue.ProjectID, &issue.ID, comment.AuthorID, "commented", "", "", bodyPreview)
	s.NotifSvc.NotifyWatchers(ctx, s.WatcherRepo, issue.ID, comment.AuthorID,
		"mention", fmt.Sprintf("New comment on %s", issue.IssueKey), bodyPreview)
	s.WsHub.PublishEvent("comment_added", issue.ProjectID, map[string]interface{}{
		"issue_id": issue.ID, "issue_key": issue.IssueKey,
		"comment_id": comment.ID, "author_id": comment.AuthorID,
	})
	return nil
}

// --- Sprint Service ---

type SprintService struct {
	SprintRepo  *repository.SprintRepo
	IssueRepo   *repository.IssueRepo
	ActivitySvc *ActivityService
	WsHub       *websocket.Hub
}

func NewSprintService(sprintRepo *repository.SprintRepo, issueRepo *repository.IssueRepo, activitySvc *ActivityService, wsHub *websocket.Hub) *SprintService {
	return &SprintService{SprintRepo: sprintRepo, IssueRepo: issueRepo, ActivitySvc: activitySvc, WsHub: wsHub}
}

func (s *SprintService) Start(ctx context.Context, sprintID uuid.UUID, userID uuid.UUID) (*models.Sprint, error) {
	sprint, err := s.SprintRepo.GetByID(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.Status != models.SprintPlanned {
		return nil, fmt.Errorf("sprint must be in 'planned' status to start, currently: %s", sprint.Status)
	}
	active, err := s.SprintRepo.GetActiveSprint(ctx, sprint.ProjectID)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, fmt.Errorf("project already has an active sprint: %s", active.Name)
	}

	now := time.Now()
	sprint.Status = models.SprintActive
	sprint.StartDate = &now
	if err := s.SprintRepo.Update(ctx, sprint); err != nil {
		return nil, err
	}

	s.ActivitySvc.Log(ctx, sprint.ProjectID, nil, userID, "sprint_started", "", "", sprint.Name)
	s.WsHub.PublishEvent("sprint_updated", sprint.ProjectID, sprint)
	return sprint, nil
}

type SprintCompletionResult struct {
	CompletedSprint  *models.Sprint `json:"completed_sprint"`
	CompletedIssues  []models.Issue `json:"completed_issues"`
	IncompleteIssues []models.Issue `json:"incomplete_issues"`
	Velocity         int            `json:"velocity"`
}

func (s *SprintService) Complete(ctx context.Context, sprintID uuid.UUID, carryOverIssueIDs []uuid.UUID, newSprintID *uuid.UUID, userID uuid.UUID) (*SprintCompletionResult, error) {
	sprint, err := s.SprintRepo.GetByID(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.Status != models.SprintActive {
		return nil, fmt.Errorf("only active sprints can be completed, currently: %s", sprint.Status)
	}

	issues, err := s.IssueRepo.ListBySprintID(ctx, sprintID)
	if err != nil {
		return nil, err
	}

	var completed, incomplete []models.Issue
	velocity := 0
	for _, issue := range issues {
		if issue.Status.Name == "Done" {
			completed = append(completed, issue)
			if issue.StoryPoints != nil {
				velocity += *issue.StoryPoints
			}
		} else {
			incomplete = append(incomplete, issue)
		}
	}

	now := time.Now()
	sprint.Status = models.SprintCompleted
	sprint.EndDate = &now
	if err := s.SprintRepo.Update(ctx, sprint); err != nil {
		return nil, err
	}

	// Carry-over selected issues to new sprint
	if len(carryOverIssueIDs) > 0 {
		if err := s.IssueRepo.BulkUpdateSprint(ctx, carryOverIssueIDs, newSprintID); err != nil {
			return nil, fmt.Errorf("failed to carry over issues: %w", err)
		}
	}

	// Move remaining incomplete to backlog
	carrySet := make(map[uuid.UUID]bool)
	for _, id := range carryOverIssueIDs {
		carrySet[id] = true
	}
	var remainingIDs []uuid.UUID
	for _, issue := range incomplete {
		if !carrySet[issue.ID] {
			remainingIDs = append(remainingIDs, issue.ID)
		}
	}
	if len(remainingIDs) > 0 {
		_ = s.IssueRepo.BulkUpdateSprint(ctx, remainingIDs, nil)
	}

	s.ActivitySvc.Log(ctx, sprint.ProjectID, nil, userID, "sprint_completed", "", sprint.Name, fmt.Sprintf("velocity: %d pts", velocity))
	s.WsHub.PublishEvent("sprint_updated", sprint.ProjectID, sprint)

	return &SprintCompletionResult{
		CompletedSprint: sprint, CompletedIssues: completed,
		IncompleteIssues: incomplete, Velocity: velocity,
	}, nil
}

// --- Search Service ---

type SearchService struct {
	Repo *repository.SearchRepo
}

func NewSearchService(repo *repository.SearchRepo) *SearchService {
	return &SearchService{Repo: repo}
}
