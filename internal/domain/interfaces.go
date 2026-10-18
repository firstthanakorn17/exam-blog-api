package domain

import "context"

type PostFilter struct {
	Status       string
	CategorySlug string
	Query        string
	IncludeDraft bool
}

type BlogRepository interface {
	GetSnapshot(ctx context.Context) (*DatabaseStore, error)
	ListPosts(ctx context.Context, filter PostFilter) ([]Post, error)
	GetPostByID(ctx context.Context, id string) (*Post, error)
	GetPostBySlug(ctx context.Context, slug string) (*Post, error)
	CreatePost(ctx context.Context, post Post) (*Post, error)
	UpdatePost(ctx context.Context, id string, post Post) (*Post, error)
	DeletePost(ctx context.Context, id string) error
	IncrementCounter(ctx context.Context, slug string, field string) (*Post, error)

	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, category Category) (*Category, error)
	DeleteCategory(ctx context.Context, id string) error

	ListCommentsBySlug(ctx context.Context, slug string) ([]CommentItem, error)
	CreateComment(ctx context.Context, comment CommentItem) (*CommentItem, error)

	GetSettings(ctx context.Context) (*SiteSettings, error)
	UpdateSettings(ctx context.Context, settings SiteSettings) (*SiteSettings, error)

	ListMedia(ctx context.Context) ([]MediaItem, error)
	CreateMedia(ctx context.Context, item MediaItem) (*MediaItem, error)
	DeleteMedia(ctx context.Context, id string) error

	ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error)
	AppendAuditLog(ctx context.Context, log AuditLog) error
}

type BlogService interface {
	GetContentSnapshot(ctx context.Context, includeDrafts bool) (*DatabaseStore, error)
	GetDashboardStats(ctx context.Context) (*DashboardStats, error)

	ListPosts(ctx context.Context, filter PostFilter) ([]Post, error)
	GetPostByID(ctx context.Context, id string) (*Post, error)
	GetPostBySlug(ctx context.Context, slug string, includeDraft bool) (*Post, error)
	CreatePost(ctx context.Context, input Post) (*Post, error)
	UpdatePost(ctx context.Context, id string, input Post) (*Post, error)
	TogglePostStatus(ctx context.Context, id string) (*Post, error)
	DeletePost(ctx context.Context, id string) error
	IncrementView(ctx context.Context, slug string) (*Post, error)
	IncrementClap(ctx context.Context, slug string) (*Post, error)

	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, input Category) (*Category, error)
	DeleteCategory(ctx context.Context, id string) error

	ListCommentsBySlug(ctx context.Context, slug string) ([]CommentItem, error)
	CreateComment(ctx context.Context, input CommentItem) (*CommentItem, error)

	GetSettings(ctx context.Context) (*SiteSettings, error)
	UpdateSettings(ctx context.Context, input SiteSettings) (*SiteSettings, error)

	ListMedia(ctx context.Context) ([]MediaItem, error)
	CreateMedia(ctx context.Context, input MediaItem) (*MediaItem, error)
	DeleteMedia(ctx context.Context, id string) error

	ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error)
	AppendAuditLog(ctx context.Context, input AuditLog) error
}
