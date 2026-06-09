# Trackr - Project Management Platform

A Jira-like project management backend built with Go, featuring a configurable workflow engine, real-time updates via WebSocket, and full-text search.

**Live Demo:** https://trackr-api-yqo6.onrender.com (free tier - may take ~30s to wake up on first request)

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| Framework | Gin |
| Database | PostgreSQL 16 |
| ORM | GORM |
| WebSocket | gorilla/websocket |
| Search | PostgreSQL tsvector + GIN index |
| Container | Docker + docker-compose |

## Architecture

```
Handler (HTTP)  →  Service (Business Logic)  →  Repository (Data Access)  →  PostgreSQL
                                               ↕
                                          WebSocket Hub (Real-time)
```

**Design Patterns:** Repository, Service Layer, Strategy (workflow actions), Observer (activity log + WebSocket)

## Quick Start

### Option 1: Use the Live Demo (fastest)
The API is live at https://trackr-api-yqo6.onrender.com with pre-seeded demo data.

A **Postman collection** (`Trackr-Postman-Collection.json`) is included in the repo with all requests organized by feature. Import it into Postman and start testing -- all requests point to the live URL by default.

> Note: Free tier sleeps after 15 min of inactivity. First request may take ~30s to wake up.

### Option 2: Run Locally with Docker
```bash
git clone https://github.com/Gaurav-Sodhani/Trackr---Project-Management-Platform.git
cd Trackr---Project-Management-Platform

# Start the app + PostgreSQL
docker-compose up --build -d

# Seed demo data (run from host machine)
cp .env.example .env
go run ./seed
```

### Option 3: Run Without Docker
```bash
# Requires PostgreSQL running locally
cp .env.example .env
# Edit .env with your PostgreSQL credentials
go run ./cmd/server
go run ./seed
```

The API is available at `http://localhost:8080`.

## API Endpoints

### Users
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/users` | Create user |
| GET | `/api/users` | List users |
| GET | `/api/users/:id` | Get user |

### Projects
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/projects` | Create project (auto-creates workflow) |
| GET | `/api/projects` | List projects |
| GET | `/api/projects/:id` | Get project |
| PATCH | `/api/projects/:id` | Update project |
| GET | `/api/projects/:id/board` | Board view (issues grouped by status) |
| GET | `/api/projects/:id/statuses` | List workflow statuses |
| GET | `/api/projects/:id/transitions` | List allowed transitions |
| POST | `/api/projects/:id/transitions` | Add transition rule |
| POST | `/api/projects/:id/custom-fields` | Define custom field |
| GET | `/api/projects/:id/custom-fields` | List custom fields |

### Issues
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/projects/:id/issues` | Create issue (auto-generates key like PROJ-1) |
| GET | `/api/projects/:id/issues` | List issues (cursor pagination) |
| GET | `/api/issues/:id` | Get issue with details |
| PATCH | `/api/issues/:id` | Update issue (optimistic locking via `version`) |
| DELETE | `/api/issues/:id` | Delete issue |
| POST | `/api/issues/:id/transitions` | **Transition status** (workflow engine) |
| POST | `/api/issues/:id/move-to-sprint` | Move to sprint or backlog |

### Sprints
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/projects/:id/sprints` | Create sprint |
| GET | `/api/projects/:id/sprints` | List sprints |
| PATCH | `/api/sprints/:id` | Update sprint |
| POST | `/api/sprints/:id/start` | Start sprint |
| POST | `/api/sprints/:id/complete` | **Complete sprint** (carry-over + velocity) |

### Comments
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/issues/:id/comments` | Add comment (auto-extracts @mentions) |
| GET | `/api/issues/:id/comments` | List threaded comments |

### Activity & Notifications
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/projects/:id/activity` | Paginated activity feed |
| GET | `/api/users/:id/notifications` | User notifications |
| PATCH | `/api/notifications/:id/read` | Mark notification read |

### Watchers
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/issues/:id/watch` | Watch issue |
| DELETE | `/api/issues/:id/watch` | Unwatch issue |
| GET | `/api/issues/:id/watchers` | List watchers |

### Search
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/search?q=OAuth` | Full-text search |
| GET | `/api/search?status=In+Progress&assignee=Bob+Chen` | Structured filters |

### WebSocket
```
ws://localhost:8080/ws?project_id=<uuid>&user_id=<uuid>&last_event_id=<optional>
```
Events: `issue_created`, `issue_updated`, `issue_moved`, `comment_added`, `sprint_updated`

## Key Scenarios

### Scenario 1: Concurrent Issue Updates
```bash
# User A updates assignee (with version=1)
curl -X PATCH /api/issues/:id -d '{"assignee_id":"...", "version": 1}'
# User B updates priority (also with version=1) → gets 409 CONFLICT
curl -X PATCH /api/issues/:id -d '{"priority":"critical", "version": 1}'
# Response: {"success":false,"error":{"code":"CONFLICT","message":"issue has been modified by another user"}}
```

### Scenario 2: Sprint Completion with Carry-Over
```bash
curl -X POST /api/sprints/:id/complete -d '{
  "carry_over_issue_ids": ["<incomplete-issue-1>", "<incomplete-issue-2>"],
  "new_sprint_id": "<next-sprint-id>",
  "user_id": "<user-id>"
}'
# Returns: completed issues, incomplete issues, velocity (story points completed)
```

### Scenario 3: Workflow Violation
```bash
# Attempt: To Do → Done directly (skipping In Progress, In Review)
curl -X POST /api/issues/:id/transitions -d '{"to_status_id":"<done-id>","user_id":"..."}'
# Response (422):
{
  "success": false,
  "error": {
    "code": "INVALID_TRANSITION",
    "message": "cannot transition from 'To Do' to 'Done'",
    "details": {
      "current_status": "To Do",
      "requested_status": "Done",
      "allowed_transitions": ["In Progress"]
    }
  }
}
```

## Project Structure

```
├── cmd/server/main.go          # Entry point, dependency wiring
├── internal/
│   ├── config/                 # Environment config
│   ├── database/               # PostgreSQL connection + FTS setup
│   ├── models/                 # GORM models (all domain entities)
│   ├── repository/             # Data access layer
│   ├── services/               # Business logic (workflow, sprints, notifications)
│   ├── handlers/               # HTTP handlers
│   ├── middleware/              # CORS, logging, error handling
│   └── websocket/              # WS hub, presence, event replay
├── seed/                       # Demo data seeder
├── docs/                       # ADR, ERD, implementation plan
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

## Design Decisions

See [docs/ADR.md](docs/ADR.md) for detailed architecture decision records.

**Key choices:**
- **Monolith** over microservices (correct for this scope)
- **PostgreSQL tsvector** over Elasticsearch (no extra infra needed)
- **In-memory WebSocket hub** with event replay buffer (production would use Redis pub/sub)
- **Optimistic locking** via version column (simple, effective for read-heavy workloads)
- **Strategy pattern** for workflow auto-actions (pluggable, extensible)

## What I'd Do With More Time

- JWT authentication + RBAC
- Redis pub/sub for multi-instance WebSocket scaling
- Elasticsearch for advanced search (fuzzy, faceted)
- Rate limiting
- Comprehensive test suite (unit + integration)
- API versioning
- Webhook support for external integrations
- File attachments on issues
- Bulk operations (move multiple issues)
