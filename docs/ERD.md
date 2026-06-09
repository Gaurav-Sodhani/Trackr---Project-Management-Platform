# Database Schema (ERD Reference)

## Entity Relationship Diagram

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   users      │     │    projects       │     │    sprints       │
├─────────────┤     ├──────────────────┤     ├─────────────────┤
│ id (PK)     │◄────│ owner_id (FK)    │     │ id (PK)         │
│ email       │     │ id (PK)          │◄────│ project_id (FK) │
│ display_name│     │ key (unique)     │     │ name            │
│ avatar_url  │     │ name             │     │ goal            │
│ created_at  │     │ description      │     │ start_date      │
│ updated_at  │     │ issue_count      │     │ end_date        │
└─────────────┘     │ created_at       │     │ status          │
       │            └──────────────────┘     └─────────────────┘
       │                    │                        │
       │                    │                        │
       │            ┌───────┴────────┐              │
       │            │                │              │
       │     ┌──────┴────────┐  ┌───┴──────────────┴──┐
       │     │workflow_status│  │      issues           │
       │     ├──────────────┤  ├──────────────────────┤
       │     │ id (PK)      │  │ id (PK)              │
       │     │ project_id   │  │ project_id (FK)      │
       │     │ name         │  │ issue_key (unique)    │
       │     │ position     │  │ type (enum)           │
       │     └──────────────┘  │ title                 │
       │            │          │ description           │
       │            │          │ status_id (FK)────────┘
       │     ┌──────┴─────────┐│ priority (enum)       │
       │     │workflow_       ││ assignee_id (FK)──────┤ users
       │     │transition      ││ reporter_id (FK)──────┤ users
       │     ├───────────────┤│ sprint_id (FK)────────┤ sprints
       │     │ id (PK)       ││ parent_id (FK)────────┤ issues (self)
       │     │ project_id    ││ story_points           │
       │     │ from_status_id││ labels (JSONB)         │
       │     │ to_status_id  ││ custom_fields (JSONB)  │
       │     │ auto_assign_  ││ version (optimistic)   │
       │     │   user_id     ││ search_vector (tsvec)  │
       │     │ required_     │└──────────────────────┘
       │     │   fields      │         │
       │     └───────────────┘         │
       │                               │
       │    ┌──────────────┐    ┌──────┴──────┐   ┌───────────────┐
       │    │  watchers     │    │  comments   │   │ activity_log  │
       │    ├──────────────┤    ├─────────────┤   ├───────────────┤
       │    │ id (PK)      │    │ id (PK)     │   │ id (PK)       │
       ├────│ user_id (FK) │    │ issue_id(FK)│   │ project_id(FK)│
       │    │ issue_id (FK)│    │ author_id   │   │ issue_id (FK) │
       │    └──────────────┘    │ parent_id   │   │ user_id (FK)  │
       │                        │ body        │   │ action        │
       │    ┌──────────────┐    │ mentions    │   │ field         │
       │    │notifications │    │ search_vec  │   │ old_value     │
       │    ├──────────────┤    └─────────────┘   │ new_value     │
       │    │ id (PK)      │                      │ metadata      │
       ├────│ user_id (FK) │                      │ created_at    │
            │ type         │                      └───────────────┘
            │ title        │
            │ issue_id(FK) │    ┌────────────────────┐
            │ read         │    │custom_field_defn    │
            └──────────────┘    ├────────────────────┤
                                │ id (PK)            │
                                │ project_id (FK)    │
                                │ name               │
                                │ field_type (enum)  │
                                │ options (JSONB)    │
                                │ required           │
                                └────────────────────┘
```

## Key Relationships
- Issue → Project (many-to-one)
- Issue → Issue (self-referential, parent_id for hierarchy)
- Issue → Sprint (many-to-one, nullable = backlog)
- Issue → WorkflowStatus (many-to-one)
- Comment → Issue (many-to-one)
- Comment → Comment (self-referential, parent_id for threading)
- WorkflowTransition → WorkflowStatus (from + to)
- All entities → User (various FKs)

## Indexes
- `issues.search_vector` → GIN index for full-text search
- `comments.search_vector` → GIN index
- `issues.project_id` → B-tree (filter by project)
- `issues.sprint_id` → B-tree (filter by sprint)
- `issues.status_id` → B-tree (board view grouping)
- `issues.parent_id` → B-tree (hierarchy queries)
- `watchers(issue_id, user_id)` → unique composite
- `activity_log.project_id` → B-tree (activity feed)
