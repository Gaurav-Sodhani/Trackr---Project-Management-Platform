package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"project-management-platform/internal/config"
	"project-management-platform/internal/models"
)

// Seed populates the database with realistic demo data.
// Run: go run ./seed
func main() {
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	// Clean existing data (order matters for foreign keys)
	for _, table := range []string{"watchers", "notifications", "activity_logs", "comments", "issues", "workflow_transitions", "workflow_statuses", "sprints", "custom_field_definitions", "projects", "users"} {
		db.Exec("DELETE FROM " + table)
	}
	// Reset sequences
	db.Exec("UPDATE projects SET issue_count = 0")

	// --- Users ---
	users := []models.User{
		{BaseModel: models.BaseModel{ID: uid("u1")}, Email: "jane@company.com", DisplayName: "Jane Smith"},
		{BaseModel: models.BaseModel{ID: uid("u2")}, Email: "bob@company.com", DisplayName: "Bob Chen"},
		{BaseModel: models.BaseModel{ID: uid("u3")}, Email: "alice@company.com", DisplayName: "Alice Kumar"},
		{BaseModel: models.BaseModel{ID: uid("u4")}, Email: "mike@company.com", DisplayName: "Mike Johnson"},
	}
	for i := range users {
		db.Create(&users[i])
	}
	fmt.Printf("created %d users\n", len(users))

	// --- Project ---
	project := models.Project{
		BaseModel:   models.BaseModel{ID: uid("p1")},
		Key:         "TRACK",
		Name:        "Trackr Platform",
		Description: "Internal project management tool for engineering teams",
		OwnerID:     uid("u1"),
	}
	db.Create(&project)

	// --- Workflow Statuses ---
	statuses := []models.WorkflowStatus{
		{BaseModel: models.BaseModel{ID: uid("s1")}, ProjectID: uid("p1"), Name: "To Do", Position: 0},
		{BaseModel: models.BaseModel{ID: uid("s2")}, ProjectID: uid("p1"), Name: "In Progress", Position: 1},
		{BaseModel: models.BaseModel{ID: uid("s3")}, ProjectID: uid("p1"), Name: "In Review", Position: 2},
		{BaseModel: models.BaseModel{ID: uid("s4")}, ProjectID: uid("p1"), Name: "Done", Position: 3},
	}
	for i := range statuses {
		db.Create(&statuses[i])
	}

	// --- Workflow Transitions (linear + backwards) ---
	transitions := []models.WorkflowTransition{
		{ProjectID: uid("p1"), FromStatusID: uid("s1"), ToStatusID: uid("s2")},                     // To Do -> In Progress
		{ProjectID: uid("p1"), FromStatusID: uid("s2"), ToStatusID: uid("s3")},                     // In Progress -> In Review
		{ProjectID: uid("p1"), FromStatusID: uid("s3"), ToStatusID: uid("s4")},                     // In Review -> Done
		{ProjectID: uid("p1"), FromStatusID: uid("s2"), ToStatusID: uid("s1")},                     // In Progress -> To Do (back)
		{ProjectID: uid("p1"), FromStatusID: uid("s3"), ToStatusID: uid("s2")},                     // In Review -> In Progress (back)
		{ProjectID: uid("p1"), FromStatusID: uid("s2"), ToStatusID: uid("s3"), AutoAssignUserID: ptr(uid("u3"))}, // auto-assign Alice as reviewer
	}
	for i := range transitions {
		db.Create(&transitions[i])
	}
	fmt.Println("created workflow: To Do -> In Progress -> In Review -> Done")

	// --- Custom Field Definitions ---
	db.Create(&models.CustomFieldDefinition{ProjectID: uid("p1"), Name: "Environment", FieldType: "dropdown", Options: models.JSONB(`["dev","staging","prod"]`)})
	db.Create(&models.CustomFieldDefinition{ProjectID: uid("p1"), Name: "Estimated Hours", FieldType: "number"})
	db.Create(&models.CustomFieldDefinition{ProjectID: uid("p1"), Name: "Due Date", FieldType: "date"})
	fmt.Println("created 3 custom field definitions")

	// --- Sprints ---
	now := time.Now()
	pastStart := now.AddDate(0, 0, -14)
	pastEnd := now.AddDate(0, 0, -1)
	sprint1 := models.Sprint{
		BaseModel: models.BaseModel{ID: uid("sp1")},
		ProjectID: uid("p1"), Name: "Sprint 1", Goal: "Authentication & User Management",
		Status: models.SprintCompleted, StartDate: &pastStart, EndDate: &pastEnd,
	}
	futureEnd := now.AddDate(0, 0, 13)
	sprint2 := models.Sprint{
		BaseModel: models.BaseModel{ID: uid("sp2")},
		ProjectID: uid("p1"), Name: "Sprint 2", Goal: "Issue tracking & board view",
		Status: models.SprintActive, StartDate: &now, EndDate: &futureEnd,
	}
	sprint3 := models.Sprint{
		BaseModel: models.BaseModel{ID: uid("sp3")},
		ProjectID: uid("p1"), Name: "Sprint 3", Goal: "Notifications & real-time",
		Status: models.SprintPlanned,
	}
	db.Create(&sprint1)
	db.Create(&sprint2)
	db.Create(&sprint3)
	fmt.Println("created 3 sprints (completed, active, planned)")

	// --- Issues ---
	// Epic
	epic := issue("i1", "p1", "TRACK-1", "epic", "User Management System", "Complete user auth and profile management", "s4", "high", "u1", "u1", nil, ptr(8), `["auth","core"]`)
	epic.SprintID = ptr(uid("sp1"))
	db.Create(&epic)

	// Stories under epic (Sprint 1 - completed)
	s1 := issue("i2", "p1", "TRACK-2", "story", "Implement OAuth 2.0 login", "Set up Google and GitHub OAuth providers", "s4", "high", "u2", "u1", ptr(uid("i1")), ptr(5), `["auth","backend"]`)
	s1.SprintID = ptr(uid("sp1"))
	db.Create(&s1)

	s2 := issue("i3", "p1", "TRACK-3", "story", "User profile page", "Display user info, avatar, settings", "s4", "medium", "u3", "u1", ptr(uid("i1")), ptr(3), `["frontend","auth"]`)
	s2.SprintID = ptr(uid("sp1"))
	db.Create(&s2)

	// Sprint 2 issues (active - mix of statuses)
	s3 := issue("i4", "p1", "TRACK-4", "story", "Issue CRUD API", "Create, read, update, delete issues", "s4", "high", "u2", "u1", nil, ptr(5), `["backend","api"]`)
	s3.SprintID = ptr(uid("sp2"))
	db.Create(&s3)

	s4 := issue("i5", "p1", "TRACK-5", "story", "Board view endpoint", "Return issues grouped by status columns", "s3", "high", "u3", "u1", nil, ptr(3), `["backend","api"]`)
	s4.SprintID = ptr(uid("sp2"))
	db.Create(&s4)

	s5 := issue("i6", "p1", "TRACK-6", "story", "Workflow engine", "Configurable status transitions with rules", "s2", "critical", "u2", "u1", nil, ptr(8), `["backend","core"]`)
	s5.SprintID = ptr(uid("sp2"))
	db.Create(&s5)

	s6 := issue("i7", "p1", "TRACK-7", "task", "Set up CI/CD pipeline", "GitHub Actions for lint, test, deploy", "s1", "medium", "u4", "u1", nil, ptr(2), `["devops"]`)
	s6.SprintID = ptr(uid("sp2"))
	db.Create(&s6)

	s7 := issue("i8", "p1", "TRACK-8", "bug", "Login redirect loop on Safari", "Users on Safari get stuck in OAuth redirect", "s2", "high", "u2", "u3", nil, ptr(2), `["bug","auth"]`)
	s7.SprintID = ptr(uid("sp2"))
	db.Create(&s7)

	// Backlog issues (no sprint)
	b1 := issue("i9", "p1", "TRACK-9", "story", "WebSocket real-time updates", "Broadcast board changes to connected clients", "s1", "high", "u4", "u1", nil, ptr(5), `["backend","websocket"]`)
	db.Create(&b1)

	b2 := issue("i10", "p1", "TRACK-10", "story", "Full-text search", "Search across issue titles and descriptions", "s1", "medium", nil, "u1", nil, ptr(3), `["backend","search"]`)
	db.Create(&b2)

	b3 := issue("i11", "p1", "TRACK-11", "task", "Write API documentation", "Swagger/OpenAPI spec for all endpoints", "s1", "low", nil, "u1", nil, ptr(2), `["docs"]`)
	db.Create(&b3)

	// Update project issue count
	db.Model(&models.Project{}).Where("id = ?", uid("p1")).Update("issue_count", 11)

	fmt.Println("created 11 issues (3 completed, 5 in sprint 2, 3 in backlog)")

	// --- Comments ---
	db.Create(&models.Comment{IssueID: uid("i6"), AuthorID: uid("u2"), Body: "Starting on the workflow engine. Planning to use Strategy pattern for transition actions."})
	db.Create(&models.Comment{IssueID: uid("i6"), AuthorID: uid("u1"), Body: "@bob looks good. Make sure we support required_fields validation before transitions."})
	db.Create(&models.Comment{IssueID: uid("i8"), AuthorID: uid("u3"), Body: "Reproduced on Safari 17. The OAuth state param is getting lost on redirect."})
	db.Create(&models.Comment{IssueID: uid("i8"), AuthorID: uid("u2"), Body: "Found it - Safari's ITP is stripping the state cookie. Will fix with SameSite=None."})
	db.Create(&models.Comment{IssueID: uid("i5"), AuthorID: uid("u3"), Body: "@jane board view is ready for review. Groups issues by status with proper ordering."})
	fmt.Println("created 5 comments")

	// --- Activity Log ---
	log.Println("seeding activity log...")
	activities := []models.ActivityLog{
		{ProjectID: uid("p1"), IssueID: ptr(uid("i2")), UserID: uid("u1"), Action: "created", NewValue: "TRACK-2"},
		{ProjectID: uid("p1"), IssueID: ptr(uid("i2")), UserID: uid("u2"), Action: "transitioned", Field: "status", OldValue: "To Do", NewValue: "In Progress"},
		{ProjectID: uid("p1"), IssueID: ptr(uid("i2")), UserID: uid("u2"), Action: "transitioned", Field: "status", OldValue: "In Progress", NewValue: "In Review"},
		{ProjectID: uid("p1"), IssueID: ptr(uid("i2")), UserID: uid("u3"), Action: "transitioned", Field: "status", OldValue: "In Review", NewValue: "Done"},
		{ProjectID: uid("p1"), IssueID: ptr(uid("i6")), UserID: uid("u1"), Action: "created", NewValue: "TRACK-6"},
		{ProjectID: uid("p1"), IssueID: ptr(uid("i6")), UserID: uid("u2"), Action: "transitioned", Field: "status", OldValue: "To Do", NewValue: "In Progress"},
		{ProjectID: uid("p1"), UserID: uid("u1"), Action: "sprint_started", NewValue: "Sprint 2"},
	}
	for i := range activities {
		activities[i].CreatedAt = time.Now().Add(-time.Duration(len(activities)-i) * time.Hour)
		db.Create(&activities[i])
	}
	fmt.Println("created 7 activity log entries")

	// --- Watchers ---
	db.Create(&models.Watcher{IssueID: uid("i6"), UserID: uid("u1")})
	db.Create(&models.Watcher{IssueID: uid("i6"), UserID: uid("u2")})
	db.Create(&models.Watcher{IssueID: uid("i8"), UserID: uid("u2")})
	db.Create(&models.Watcher{IssueID: uid("i8"), UserID: uid("u3")})
	fmt.Println("created 4 watchers")

	fmt.Println("\n--- seed complete ---")
	fmt.Println("Project: TRACK (Trackr Platform)")
	fmt.Println("Users: jane, bob, alice, mike")
	fmt.Println("Workflow: To Do -> In Progress -> In Review -> Done")
	fmt.Println("Sprints: 1 (completed), 2 (active), 3 (planned)")
	fmt.Println("Issues: 11 total across sprints + backlog")
}

// uid generates a deterministic UUID from a short key (for seed reproducibility).
func uid(key string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("trackr-seed-"+key))
}

func ptr[T any](v T) *T { return &v }

func issue(key, projKey, issueKey, itype, title, desc, statusKey, priority string, assignee interface{}, reporter string, parentID *uuid.UUID, sp *int, labels string) models.Issue {
	i := models.Issue{
		BaseModel:   models.BaseModel{ID: uid(key)},
		ProjectID:   uid(projKey),
		IssueKey:    issueKey,
		Type:        itype,
		Title:       title,
		Description: desc,
		StatusID:    uid(statusKey),
		Priority:    priority,
		ReporterID:  uid(reporter),
		ParentID:    parentID,
		StoryPoints: sp,
		Labels:      models.JSONB(labels),
		Version:     1,
	}
	switch a := assignee.(type) {
	case string:
		id := uid(a)
		i.AssigneeID = &id
	case *uuid.UUID:
		i.AssigneeID = a
	}
	return i
}
