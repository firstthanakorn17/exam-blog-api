# exam-blog-api

Go Fiber v2 Backend API for the **ExamBlog Platform** (`exam-blog-console` + `exam-blog-interface`), built with **Clean Layered Architecture**.

## Project Structure (Clean Layered Architecture)

```text
exam-blog-api/
├── cmd/
│   └── api/
│       └── main.go                       # Entrypoint (:4000), Dependency Injection & Graceful Shutdown
├── internal/
│   ├── config/
│   │   └── config.go                     # Environment configuration loader
│   ├── domain/
│   │   ├── models.go                     # Core Entities (Post, Category, CommentItem, SiteSettings, etc.)
│   │   └── interfaces.go                 # BlogRepository & BlogService contracts
│   ├── repository/
│   │   └── json_repository.go            # Thread-safe (sync.RWMutex) persistent storage implementation
│   ├── service/
│   │   └── blog_service.go               # Business logic, audit logging, and cache revalidation triggers
│   ├── handler/
│   │   ├── router.go                     # Fiber route registration (/api/v1/...)
│   │   ├── post_handler.go               # Posts CRUD, status toggle, views & claps
│   │   ├── category_comment_handler.go   # Categories & Comments endpoints
│   │   └── system_handler.go             # Snapshot, stats, settings, media & audit logs
│   ├── middleware/
│   │   └── middleware.go                 # Fiber Recover, RequestID, Logger & CORS
│   └── webhook/
│       └── notifier.go                   # Async Next.js 16 cacheTag revalidation webhook dispatcher
├── go.mod
└── README.md
```

## Quick Start

```bash
go run ./cmd/api
# Listening on http://localhost:4000
```

## REST Endpoints (`/api/v1`)

- `GET  /health` — Service health check
- `GET  /api/v1/content` — Full content snapshot (`?draft=1` or `?slug=<slug>`)
- `POST /api/v1/content` — Reader actions (`increment_view`, `clap`, `add_comment`)
- `GET  /api/v1/stats` — Dashboard KPI statistics
- `GET  /api/v1/posts` — Filterable posts list (`?status=`, `?category=`, `?q=`, `?draft=1`)
- `GET  /api/v1/posts/:idOrSlug` — Get post by ID or slug
- `POST /api/v1/posts` — Create post + trigger Next.js `cacheTag` webhook
- `PUT  /api/v1/posts/:id` — Update post + trigger Next.js `cacheTag` webhook
- `PATCH /api/v1/posts/:id/toggle-status` — Toggle draft/published
- `DELETE /api/v1/posts/:id` — Delete post
- `POST /api/v1/posts/:slug/view` — Increment post views
- `POST /api/v1/posts/:slug/clap` — Increment post claps
- `GET  /api/v1/categories` / `POST /api/v1/categories` / `DELETE /api/v1/categories/:id`
- `GET  /api/v1/comments?slug=` / `POST /api/v1/comments`
- `GET  /api/v1/settings` / `PUT /api/v1/settings`
- `GET  /api/v1/media` / `POST /api/v1/media` / `DELETE /api/v1/media/:id`
- `GET  /api/v1/audit-logs`
