# Architecture Decision Record (ADR)

## ADR-001: Tech Stack Selection

**Status:** Accepted  
**Date:** 2026-06-09

### Context
Take-home assignment for Swiggy SDE-1 Backend: Build a Jira-like project management platform backend.
Both existing Swiggy applications (app-002, app-040) were for Go roles. Assignment doesn't mandate a language.
Deadline: ~6 hours from start.

### Decision
- **Language:** Go (Gin framework)
- **Database:** PostgreSQL 16
- **ORM:** GORM
- **WebSocket:** gorilla/websocket
- **API Docs:** Swagger via swaggo/swag
- **Containerization:** Docker + docker-compose
- **Deployment:** Render (free tier)

### Rationale
| Choice | Why |
|--------|-----|
| Go | Aligns with Swiggy's tech stack. Both prior applications were for Go roles. Go's goroutine model is ideal for WebSocket concurrency. |
| PostgreSQL | Relational data (projects, issues, sprints) needs referential integrity. PG supports JSONB for custom fields and tsvector for full-text search natively. |
| GORM | Most popular Go ORM. Auto-migration, preloading, soft deletes. Reduces boilerplate. |
| Gin | Fastest Go HTTP framework. Clean middleware model, binding/validation built-in. |
| gorilla/websocket | De facto Go WebSocket library. Production-tested, minimal API. |

---

## ADR-002: Architecture Pattern

### Decision
Layered architecture: **Handler → Service → Repository → Database**

```
Handler (HTTP layer)     → parses requests, returns responses
Service (business logic) → workflow rules, sprint logic, notifications
Repository (data access) → GORM queries, no business logic
```

### Why not microservices?
Monolith is correct for this scope. A single service with clean internal boundaries. 
Microservices would be over-engineering for a take-home and add deployment complexity.

### Design Patterns Used
| Pattern | Where | Why |
|---------|-------|-----|
| Repository | Data access layer | Decouples business logic from ORM details |
| Service Layer | Business logic | Keeps handlers thin, logic testable |
| Strategy | Workflow engine | Transition auto-actions are pluggable (auto-assign, validate fields) |
| Observer | Activity log + WebSocket | Mutations trigger observers (logger, WS broadcaster) |

---

## ADR-003: Workflow Engine Design

### Decision
Configurable per-project workflow with transition rules stored in the database.

**How it works:**
1. Each project has ordered `WorkflowStatus` rows (To Do → In Progress → In Review → Done)
2. `WorkflowTransition` rows define allowed moves (from_status → to_status)
3. Each transition can have:
   - `required_fields`: JSON array of fields that must be non-empty before transition
   - `auto_assign_user_id`: user to auto-assign on this transition
4. If a transition row doesn't exist for (from → to), the API returns 422 with allowed transitions

**Extensibility:** Adding a new auto-action = adding a new field to WorkflowTransition or implementing a new action handler. No existing code changes needed.

---

## ADR-004: Optimistic Locking for Concurrent Updates

### Decision
`version` column on issues. Updates use `WHERE id = ? AND version = ?`. If 0 rows affected → conflict.

**Why not pessimistic locking?** 
- Take-home scope doesn't need distributed locks
- Optimistic locking is simpler, performs better for read-heavy workloads
- Matches the assignment's Scenario 1 requirements

---

## ADR-005: Full-Text Search via PostgreSQL

### Decision
Use PostgreSQL's native `tsvector` + GIN index instead of Elasticsearch.

**Why:**
- No additional infrastructure needed
- Sufficient for the scope (searching issues by title/description/comments)
- Weighted search: title matches rank higher than description matches (setweight A vs B)
- Auto-updated via trigger on INSERT/UPDATE

**Trade-off:** Won't scale to millions of issues like ES would, but perfect for this assignment's scope.

---

## ADR-006: Real-Time via WebSocket Hub

### Decision
In-memory WebSocket hub with goroutine-based event loop.

**Components:**
- Hub: manages connections per project, broadcasts events
- Client: single WS connection with send channel
- Event types: issue_created, issue_updated, issue_moved, comment_added, sprint_updated

**Presence:** In-memory map of project_id → user_id → last_seen timestamp.

**Reconnection:** Ring buffer of last 100 events per project. On reconnect with `last_event_id`, server replays missed events.

**Trade-off:** In-memory = lost on restart. Production would use Redis pub/sub. Documented in "what I'd do with more time."
