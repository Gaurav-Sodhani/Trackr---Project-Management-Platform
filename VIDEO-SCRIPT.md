# Video Walkthrough Script - Trackr
# Total: 8-10 minutes
# Tools open: VS Code (project folder), Postman (collection imported), Browser (Render dashboard)

---

## BEFORE RECORDING
- Open VS Code with the project folder
- Open Postman with the collection imported
- Hit "1a. List Users" once to wake up the Render server (takes 30s on first hit)
- Open browser tab with Render dashboard (shows deployment is live)
- Reset seed data if needed: `DATABASE_URL="..." go run ./seed`

---

## INTRO (30 seconds)

[Screen: VS Code with project open]

"Hey, so I built Trackr -- a project management platform backend, similar to Jira. 
It's built in Go using the Gin framework, with PostgreSQL as the database. 
Let me walk you through the architecture first, then I'll demo the working system."

---

## PART 1: ARCHITECTURE (2-3 minutes)

[Screen: VS Code - show the file explorer sidebar]

"So the project follows a clean layered architecture."

[Click through folders as you mention them]

"We have:
- cmd/server -- that's the entry point
- internal/models -- all the database entities
- internal/handlers -- HTTP layer, parses requests
- internal/services -- business logic, this is where the workflow engine lives
- internal/repository -- data access, all the database queries
- internal/websocket -- real-time event broadcasting
- internal/middleware -- CORS, logging, error handling"

[Open cmd/server/main.go]

"The main file does dependency injection manually. Repositories are created first, 
then services that use those repositories, then handlers that use the services. 
No magic frameworks -- everything is explicit and easy to trace."

[Open internal/models/models.go, scroll to Issue struct around line 110]

"This is the Issue model -- the core entity. A few things to note:
- ParentID lets us build hierarchy -- epics contain stories, stories contain sub-tasks
- SprintID is nullable -- if it's nil, the issue is in the backlog
- Labels and CustomFields are stored as JSONB -- flexible without schema changes
- And this Version field -- that's for optimistic locking, I'll demo that in a bit"

[Open internal/services/services.go, scroll to ValidateAndTransition around line 108]

"This is the heart of the system -- the workflow engine. When someone tries to move 
an issue from one status to another, this function:
- First checks if that transition is even allowed in the database
- If not, returns an error with the list of what IS allowed
- Then checks if required fields are filled -- like assignee must be set before review
- Then logs it, notifies watchers, and broadcasts via WebSocket

The key design choice here -- transitions are stored as data in the database, not 
hardcoded. So adding a new workflow rule is just inserting a row, no code change needed."

---

## PART 2: DEMO - EXISTING DATA (1-2 minutes)

[Switch to Postman]

"Let me show the working system. This is hitting the live deployed version on Render."

[Run 1a. List Users]
"We have 4 users seeded -- Jane, Bob, Alice, Mike."

[Run 1b. List Projects]
"One project called Trackr with the key TRACK."

[Run 1c. Board View]
"This is the board view -- issues grouped by status columns. 
You can see Done has 4 issues, In Progress has 2, In Review has 1, To Do has 4. 
This is what would power the Kanban board on the frontend."

[Run 1d. Workflow Statuses]
"Each project has its own configurable statuses -- To Do, In Progress, In Review, Done. 
The position field controls the display order."

[Run 1f. Sprints]
"Three sprints -- Sprint 1 is completed, Sprint 2 is active, Sprint 3 is planned."

---

## PART 3: CREATING THINGS LIVE (1 minute)

[Run 2a. Create New User]
"I can create a user on the fly."

[Run 2b. Create New Issue]
"And create an issue -- notice it auto-generated the key TRACK-12, 
defaulted to the first status which is To Do, and the version starts at 1."

---

## PART 4: SCENARIO 3 - WORKFLOW RULES (1-1.5 minutes)

"Now the interesting part -- the workflow engine."

[Run 3a. ILLEGAL: To Do → Done]
"I'm trying to move this issue from To Do directly to Done, skipping the middle steps."

[Point at the response]
"And it fails -- 422, 'cannot transition from To Do to Done'. 
And it tells me exactly what I CAN do -- move to In Progress. 
This is all configured in the database, not hardcoded."

[Run 3b. LEGAL: To Do → In Progress]
"Now if I do the valid move -- To Do to In Progress -- it works. 
Status updated, version bumped to 2, activity logged automatically."

---

## PART 5: SCENARIO 1 - CONCURRENT UPDATES (1 minute)

"What happens when two people edit the same issue at the same time?"

[Run 4a. Get Issue]
"Let me grab an issue -- note the version is 1."

[Run 4b. Update with WRONG version]
"Now imagine User A already updated this. I'm sending version 999 which doesn't match. 
And we get 409 Conflict -- 'issue has been modified by another user'. 
The client knows to re-fetch and retry."

[Run 4c. Update with CORRECT version]
"With the correct version, the update goes through and version bumps to 2."

---

## PART 6: COLLABORATION (1-1.5 minutes)

[Run 5a. Add Comment with @mention]
"Comments support @mentions. I'm writing '@bob' and '@alice' in the comment body."

[Point at response]
"See the mentions field -- it automatically extracted 'bob' and 'alice' from the text. 
This would trigger notifications for them."

[Run 5c. Activity Feed]
"Every action gets logged automatically -- created, transitioned, commented. 
This is the full audit trail. Paginated with cursor-based pagination."

[Run 5f. User Notifications]
"And Bob has notifications -- he was assigned issues, mentioned in comments, 
things he's watching got updated."

---

## PART 7: SEARCH (30-45 seconds)

[Run 6a. Full-Text Search]
"Search for 'OAuth' -- it finds matching issues using PostgreSQL's native 
full-text search with weighted ranking. Title matches rank higher than description."

[Run 6b. Structured Filter]
"Can also filter by structured fields -- here's all high priority issues."

---

## PART 8: OTHER FEATURES (30 seconds)

"A few more things I built that are worth mentioning:
- WebSocket endpoint for real-time updates -- clients get events when anything changes on the board
- Presence tracking -- the server knows who's viewing which board
- Reconnection support -- if a client disconnects, it can replay missed events
- Custom fields per project -- each project can define its own text, number, dropdown, or date fields
- Watchers -- subscribe to issues and get notified on changes"

---

## PART 9: WHAT I'D DO WITH MORE TIME (45 seconds)

"If I had more time, I'd add:
- JWT authentication with role-based access control
- Redis pub/sub so the WebSocket layer scales across multiple server instances
- Elasticsearch for more advanced search -- fuzzy matching, faceted search
- A comprehensive test suite -- unit and integration tests
- Rate limiting on the API
- And webhook support so external tools can integrate with status changes"

---

## CLOSING (15 seconds)

"That's Trackr -- a fully functional project management backend with a configurable 
workflow engine, real-time updates, full-text search, and sprint management. Thanks!"

---

## TIPS FOR RECORDING
- Use OBS Studio (free) or Windows Game Bar (Win+G) to record
- Keep VS Code and Postman side by side or switch between them
- Don't rush -- pause briefly between sections
- If a Render request takes long to respond, say "the free tier takes a moment to wake up"
- Talk naturally -- you don't need to memorize this word for word
- Its okay to glance at the collection names to know what to click next
