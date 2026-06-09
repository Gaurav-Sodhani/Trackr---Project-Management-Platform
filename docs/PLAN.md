# Implementation Plan

## Assignment Requirements Checklist

### 1. Data Model & Storage Layer
- [ ] Projects, Issues (Epic/Story/Task/Bug/Sub-task), Sprints, Users, Comments, Activity Log
- [ ] Parent-child relationships (Epic → Story → Sub-task)
- [ ] Full audit trail for every issue mutation
- [ ] Custom fields per project (text, number, dropdown, date)
- [ ] Proper indexing for high throughput

### 2. Issue & Workflow Engine
- [ ] Configurable status columns per project
- [ ] Transition rules (allowed transitions only)
- [ ] Automatic actions on transitions (auto-assign reviewer)
- [ ] Validation hooks (block if required fields missing)
- [ ] Sprint CRUD with date ranges
- [ ] Move issues between backlog and active sprint
- [ ] Sprint completion with carry-over
- [ ] Sprint velocity tracking

### 3. Collaboration APIs
- [ ] Threaded comments with @mentions
- [ ] Paginated, filterable activity feed
- [ ] Notification system (assignments, mentions, status changes)
- [ ] Watcher subscribe/unsubscribe

### 4. Real-Time Sync
- [ ] WebSocket broadcasting board state changes
- [ ] Event types: issue_created, issue_updated, issue_moved, comment_added, sprint_updated
- [ ] Presence tracking (who is viewing)
- [ ] Reconnection + missed event replay

### 5. Search & Filtering
- [ ] Full-text search (titles, descriptions, comments)
- [ ] Structured query (status + assignee + priority)
- [ ] Proper indexing (GIN index on tsvector)
- [ ] Cursor-based pagination

### Deliverables
- [ ] GitHub repo with clean structure
- [ ] README: architecture, setup, API docs
- [ ] Docker/docker-compose for local dev
- [ ] Hosted demo on Render
- [ ] All endpoints functional
- [ ] Migrations + seed data
- [ ] Swagger docs
- [ ] Architecture decisions (docs/ADR.md)
- [ ] ERD diagram

### Scenarios
- [ ] Scenario 1: Concurrent updates (optimistic locking, 409 on conflict)
- [ ] Scenario 2: Sprint completion with carry-over + velocity
- [ ] Scenario 3: Workflow violation → 422 with allowed transitions

## Commit Plan

| Checkpoint | What's Included |
|------------|----------------|
| C1: Scaffolding | Project structure, models, docker-compose, migrations, docs |
| C2: CRUD APIs | Users, Projects, Issues, Sprints, Comments CRUD |
| C3: Workflow Engine | Transition rules, validation hooks, auto-actions |
| C4: Sprint Lifecycle | Start/complete sprint, carry-over, velocity |
| C5: Collaboration | Activity log, notifications, watchers, search |
| C6: WebSocket | Real-time broadcast, presence, reconnection replay |
| C7: Final | Seed data, optimistic locking, README, deployment |
