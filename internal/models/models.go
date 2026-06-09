package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// --- Enums / Constants ---

const (
	IssueTypeEpic    = "epic"
	IssueTypeStory   = "story"
	IssueTypeTask    = "task"
	IssueTypeBug     = "bug"
	IssueTypeSubtask = "subtask"
)

const (
	PriorityLow      = "low"
	PriorityMedium   = "medium"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

const (
	SprintPlanned   = "planned"
	SprintActive    = "active"
	SprintCompleted = "completed"
)

// --- JSONB helper ---

// JSONB wraps json.RawMessage to work with GORM's postgres jsonb columns.
type JSONB json.RawMessage

func (j JSONB) Value() (interface{}, error) {
	if len(j) == 0 {
		return "[]", nil
	}
	return string(j), nil
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = JSONB("[]")
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		s, ok := value.(string)
		if !ok {
			return nil
		}
		bytes = []byte(s)
	}
	*j = bytes
	return nil
}

func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}
	return json.RawMessage(j).MarshalJSON()
}

func (j *JSONB) UnmarshalJSON(data []byte) error {
	*j = data
	return nil
}

// --- Base hook: auto-generate UUIDs ---

type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// --- Core Domain Models ---

type User struct {
	BaseModel
	Email       string `gorm:"uniqueIndex;not null" json:"email"`
	DisplayName string `gorm:"not null" json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type Project struct {
	BaseModel
	Key         string `gorm:"uniqueIndex;size:10;not null" json:"key"` // e.g. "PROJ"
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description,omitempty"`
	OwnerID     uuid.UUID `gorm:"type:uuid;not null" json:"owner_id"`
	Owner       User      `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	IssueCount  int       `gorm:"default:0" json:"-"` // tracks next issue number
}

type Issue struct {
	BaseModel
	ProjectID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"project_id"`
	IssueKey     string     `gorm:"uniqueIndex;not null" json:"issue_key"` // "PROJ-1"
	Type         string     `gorm:"not null" json:"type"`                  // epic/story/task/bug/subtask
	Title        string     `gorm:"not null" json:"title"`
	Description  string     `json:"description,omitempty"`
	StatusID     uuid.UUID  `gorm:"type:uuid;not null" json:"status_id"`
	Status       WorkflowStatus `gorm:"foreignKey:StatusID" json:"status,omitempty"`
	Priority     string     `gorm:"default:'medium'" json:"priority"`
	AssigneeID   *uuid.UUID `gorm:"type:uuid;index" json:"assignee_id,omitempty"`
	Assignee     *User      `gorm:"foreignKey:AssigneeID" json:"assignee,omitempty"`
	ReporterID   uuid.UUID  `gorm:"type:uuid;not null" json:"reporter_id"`
	Reporter     User       `gorm:"foreignKey:ReporterID" json:"reporter,omitempty"`
	SprintID     *uuid.UUID `gorm:"type:uuid;index" json:"sprint_id,omitempty"` // nil = backlog
	ParentID     *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"` // for hierarchy
	Children     []Issue    `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	StoryPoints  *int       `json:"story_points,omitempty"`
	Labels       JSONB      `gorm:"type:jsonb;default:'[]'" json:"labels"`
	CustomFields JSONB      `gorm:"type:jsonb;default:'{}'" json:"custom_fields"`
	Version      int        `gorm:"default:1;not null" json:"version"` // optimistic locking
}

type Sprint struct {
	BaseModel
	ProjectID uuid.UUID  `gorm:"type:uuid;not null;index" json:"project_id"`
	Name      string     `gorm:"not null" json:"name"`
	Goal      string     `json:"goal,omitempty"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Status    string     `gorm:"default:'planned'" json:"status"` // planned/active/completed
}

type Comment struct {
	BaseModel
	IssueID  uuid.UUID  `gorm:"type:uuid;not null;index" json:"issue_id"`
	AuthorID uuid.UUID  `gorm:"type:uuid;not null" json:"author_id"`
	Author   User       `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	ParentID *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"` // threading
	Body     string     `gorm:"not null" json:"body"`
	Mentions JSONB      `gorm:"type:jsonb;default:'[]'" json:"mentions"` // extracted @user_ids
	Replies  []Comment  `gorm:"foreignKey:ParentID" json:"replies,omitempty"`
}

// --- Workflow Models ---

type WorkflowStatus struct {
	BaseModel
	ProjectID uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Name      string    `gorm:"not null" json:"name"` // "To Do", "In Progress", etc.
	Position  int       `gorm:"not null" json:"position"` // display order
}

// WorkflowTransition defines an allowed status change within a project.
// If no transition row exists for (from -> to), the move is blocked.
type WorkflowTransition struct {
	BaseModel
	ProjectID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"project_id"`
	FromStatusID uuid.UUID  `gorm:"type:uuid;not null" json:"from_status_id"`
	FromStatus   WorkflowStatus `gorm:"foreignKey:FromStatusID" json:"from_status,omitempty"`
	ToStatusID   uuid.UUID  `gorm:"type:uuid;not null" json:"to_status_id"`
	ToStatus     WorkflowStatus `gorm:"foreignKey:ToStatusID" json:"to_status,omitempty"`
	// Auto-action: assign this user on transition (e.g., auto-assign reviewer)
	AutoAssignUserID *uuid.UUID `gorm:"type:uuid" json:"auto_assign_user_id,omitempty"`
	// Required fields (JSON array of field names) that must be non-empty before transition
	RequiredFields JSONB `gorm:"type:jsonb;default:'[]'" json:"required_fields"`
}

// --- Activity & Collaboration ---

type ActivityLog struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	IssueID   *uuid.UUID `gorm:"type:uuid;index" json:"issue_id,omitempty"`
	UserID    uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Action    string    `gorm:"not null" json:"action"` // created, updated, transitioned, commented, etc.
	Field     string    `json:"field,omitempty"`         // which field changed
	OldValue  string    `json:"old_value,omitempty"`
	NewValue  string    `json:"new_value,omitempty"`
	Metadata  JSONB     `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *ActivityLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Type      string    `gorm:"not null" json:"type"` // assignment, mention, status_change, watcher
	Title     string    `gorm:"not null" json:"title"`
	Message   string    `json:"message"`
	IssueID   *uuid.UUID `gorm:"type:uuid" json:"issue_id,omitempty"`
	Read      bool      `gorm:"default:false" json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

type Watcher struct {
	ID      uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	IssueID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_watcher_issue_user" json:"issue_id"`
	UserID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_watcher_issue_user" json:"user_id"`
}

func (w *Watcher) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// --- Custom Fields ---

// CustomFieldDefinition lets each project define its own fields.
type CustomFieldDefinition struct {
	BaseModel
	ProjectID uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	Name      string    `gorm:"not null" json:"name"`
	FieldType string    `gorm:"not null" json:"field_type"` // text, number, dropdown, date
	Options   JSONB     `gorm:"type:jsonb;default:'[]'" json:"options,omitempty"` // dropdown choices
	Required  bool      `gorm:"default:false" json:"required"`
}
