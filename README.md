# exam-blog-api

Go Fiber v2 Backend API for the **ExamBlog Platform** (`exam-blog-console` + `exam-blog-interface`), built with **Clean Layered Architecture**, **PostgreSQL 16 (`pgxpool`)**, and **Redis 7 (`go-redis/v9` Cache-Aside Decorator)**.

## Project Structure (Clean Layered Architecture)

```text
exam-blog-api/
├── cmd/
│   └── api/
│       └── main.go                       # Entrypoint (:4000), Postgres + Redis DI & Graceful Shutdown
├── internal/
│   ├── config/
│   │   └── config.go                     # Environment configuration loader (DATABASE_URL, REDIS_ADDR, etc.)
│   ├── domain/
│   │   ├── models.go                     # Core Entities (Post, Category, CommentItem, SiteSettings, etc.)
│   │   └── interfaces.go                 # BlogRepository & BlogService contracts
│   ├── repository/
│   │   ├── postgres_repository.go        # PostgreSQL 16 repository (pgx/v5 pool + auto-migration + seed)
│   │   └── json_repository.go            # Fallback JSON file repository
│   ├── cache/
│   │   └── redis_repository.go           # Redis 7 Cache-Aside Decorator wrapping BlogRepository
│   ├── service/
│   │   └── blog_service.go               # Business logic, audit logging, and cache revalidation triggers
│   ├── handler/
│   │   ├── router.go                     # Fiber route registration (/api/v1/...)
│   │   ├── post_handler.go               # Posts CRUD, status toggle, views & claps
│   │   ├── category_comment_handler.go   # Categories & Comments endpoints
│   │   └── system_handler.go             # Health (Postgres + Redis status), Snapshot, Stats, Settings
│   ├── middleware/
│   │   └── middleware.go                 # Fiber Recover, RequestID, Logger & CORS
│   └── webhook/
│       └── notifier.go                   # Async Next.js 16 cacheTag revalidation webhook dispatcher
├── docker-compose.yml                    # Resource-tuned PostgreSQL 16 Alpine + Redis 7 Alpine
├── Makefile
├── go.mod
└── README.md
```

## Quick Start

```bash
# 1. Start PostgreSQL 16 (:5432) and Redis 7 (:6379)
make db-up

# 2. Run Go Fiber API (:4000)
make run
```
