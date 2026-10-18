package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool     *pgxpool.Pool
	seedFile string
}

func NewPostgresRepository(ctx context.Context, databaseURL, seedFile string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	repo := &PostgresRepository{
		pool:     pool,
		seedFile: seedFile,
	}
	if err := repo.migrateAndSeed(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return repo, nil
}

func (r *PostgresRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *PostgresRepository) migrateAndSeed(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS categories (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	parent_slug TEXT,
	description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS posts (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	excerpt TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	category_slug TEXT NOT NULL DEFAULT 'engineering',
	status TEXT NOT NULL DEFAULT 'draft',
	featured BOOLEAN NOT NULL DEFAULT FALSE,
	views INTEGER NOT NULL DEFAULT 0,
	claps INTEGER NOT NULL DEFAULT 12,
	author TEXT NOT NULL DEFAULT 'Admin First',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	seo_title TEXT NOT NULL DEFAULT '',
	seo_description TEXT NOT NULL DEFAULT '',
	focus_keyword TEXT NOT NULL DEFAULT '',
	canonical_url TEXT NOT NULL DEFAULT '',
	no_index BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);
CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
CREATE INDEX IF NOT EXISTS idx_posts_category_slug ON posts(category_slug);

CREATE TABLE IF NOT EXISTS comments (
	id TEXT PRIMARY KEY,
	post_slug TEXT NOT NULL,
	author TEXT NOT NULL,
	body TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_comments_post_slug ON comments(post_slug);

CREATE TABLE IF NOT EXISTS media (
	id TEXT PRIMARY KEY,
	filename TEXT NOT NULL,
	url TEXT NOT NULL,
	size_bytes BIGINT NOT NULL DEFAULT 0,
	mime_type TEXT NOT NULL DEFAULT '',
	alt TEXT NOT NULL DEFAULT '',
	uploaded_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
	id TEXT PRIMARY KEY,
	action TEXT NOT NULL,
	target TEXT NOT NULL,
	actor TEXT NOT NULL,
	timestamp TEXT NOT NULL,
	meta TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS site_settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	site_name TEXT NOT NULL,
	tagline TEXT NOT NULL,
	posts_per_page INTEGER NOT NULL DEFAULT 10,
	maintenance_mode BOOLEAN NOT NULL DEFAULT FALSE,
	allow_comments BOOLEAN NOT NULL DEFAULT TRUE,
	default_seo_description TEXT NOT NULL DEFAULT '',
	site_url TEXT NOT NULL DEFAULT '',
	twitter_handle TEXT NOT NULL DEFAULT ''
);
`
	if _, err := r.pool.Exec(ctx, schema); err != nil {
		return err
	}

	var count int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Load initial seed from shared JSON file or fallback seed
	store := defaultSeedStore()
	if r.seedFile != "" {
		if raw, err := os.ReadFile(r.seedFile); err == nil {
			_ = json.Unmarshal(raw, &store)
		}
	}
	if len(store.Comments) == 0 {
		store.Comments = defaultComments()
	}

	for _, c := range store.Categories {
		_, _ = r.CreateCategory(ctx, c)
	}
	for _, p := range store.Posts {
		if p.Claps == 0 {
			p.Claps = 18
		}
		_, _ = r.CreatePost(ctx, p)
	}
	for _, cmt := range store.Comments {
		_, _ = r.CreateComment(ctx, cmt)
	}
	for _, m := range store.Media {
		_, _ = r.CreateMedia(ctx, m)
	}
	for _, logEntry := range store.AuditLogs {
		_ = r.AppendAuditLog(ctx, logEntry)
	}
	_, _ = r.UpdateSettings(ctx, store.Settings)

	return nil
}

func (r *PostgresRepository) GetSnapshot(ctx context.Context) (*domain.DatabaseStore, error) {
	posts, err := r.ListPosts(ctx, domain.PostFilter{IncludeDraft: true})
	if err != nil {
		return nil, err
	}
	categories, err := r.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	comments, err := r.ListCommentsBySlug(ctx, "")
	if err != nil {
		return nil, err
	}
	media, err := r.ListMedia(ctx)
	if err != nil {
		return nil, err
	}
	logs, err := r.ListAuditLogs(ctx, 50)
	if err != nil {
		return nil, err
	}
	settings, err := r.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	return &domain.DatabaseStore{
		Posts:      posts,
		Categories: categories,
		Comments:   comments,
		Media:      media,
		AuditLogs:  logs,
		Settings:   *settings,
	}, nil
}

func (r *PostgresRepository) ListPosts(ctx context.Context, filter domain.PostFilter) ([]domain.Post, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, slug, excerpt, content, category_slug, status, featured,
		       views, claps, author, updated_at, created_at, seo_title, seo_description,
		       focus_keyword, canonical_url, no_index
		FROM posts
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	q := strings.ToLower(strings.TrimSpace(filter.Query))
	var out []domain.Post
	for rows.Next() {
		var p domain.Post
		var status string
		if err := rows.Scan(
			&p.ID, &p.Title, &p.Slug, &p.Excerpt, &p.Content, &p.CategorySlug, &status,
			&p.Featured, &p.Views, &p.Claps, &p.Author, &p.UpdatedAt, &p.CreatedAt,
			&p.SeoTitle, &p.SeoDescription, &p.FocusKeyword, &p.CanonicalUrl, &p.NoIndex,
		); err != nil {
			return nil, err
		}
		p.Status = domain.PostStatus(status)

		if !filter.IncludeDraft && filter.Status == "" && p.Status != domain.StatusPublished {
			continue
		}
		if filter.Status != "" && filter.Status != "all" && string(p.Status) != filter.Status {
			continue
		}
		if filter.CategorySlug != "" && filter.CategorySlug != "all" {
			if p.CategorySlug != filter.CategorySlug && !strings.HasPrefix(p.CategorySlug, filter.CategorySlug+"/") {
				continue
			}
		}
		if q != "" {
			match := strings.Contains(strings.ToLower(p.Title), q) ||
				strings.Contains(strings.ToLower(p.Excerpt), q) ||
				strings.Contains(strings.ToLower(p.Author), q) ||
				strings.Contains(strings.ToLower(p.FocusKeyword), q)
			if !match {
				continue
			}
		}
		out = append(out, p)
	}
	if out == nil {
		out = []domain.Post{}
	}
	return out, nil
}

func (r *PostgresRepository) GetPostByID(ctx context.Context, id string) (*domain.Post, error) {
	var p domain.Post
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, slug, excerpt, content, category_slug, status, featured,
		       views, claps, author, updated_at, created_at, seo_title, seo_description,
		       focus_keyword, canonical_url, no_index
		FROM posts WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Title, &p.Slug, &p.Excerpt, &p.Content, &p.CategorySlug, &status,
		&p.Featured, &p.Views, &p.Claps, &p.Author, &p.UpdatedAt, &p.CreatedAt,
		&p.SeoTitle, &p.SeoDescription, &p.FocusKeyword, &p.CanonicalUrl, &p.NoIndex,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Status = domain.PostStatus(status)
	return &p, nil
}

func (r *PostgresRepository) GetPostBySlug(ctx context.Context, slug string) (*domain.Post, error) {
	var p domain.Post
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, slug, excerpt, content, category_slug, status, featured,
		       views, claps, author, updated_at, created_at, seo_title, seo_description,
		       focus_keyword, canonical_url, no_index
		FROM posts WHERE slug = $1
	`, slug).Scan(
		&p.ID, &p.Title, &p.Slug, &p.Excerpt, &p.Content, &p.CategorySlug, &status,
		&p.Featured, &p.Views, &p.Claps, &p.Author, &p.UpdatedAt, &p.CreatedAt,
		&p.SeoTitle, &p.SeoDescription, &p.FocusKeyword, &p.CanonicalUrl, &p.NoIndex,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Status = domain.PostStatus(status)
	return &p, nil
}

func (r *PostgresRepository) CreatePost(ctx context.Context, p domain.Post) (*domain.Post, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO posts (
			id, title, slug, excerpt, content, category_slug, status, featured,
			views, claps, author, updated_at, created_at, seo_title, seo_description,
			focus_keyword, canonical_url, no_index
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (id) DO NOTHING
	`,
		p.ID, p.Title, p.Slug, p.Excerpt, p.Content, p.CategorySlug, string(p.Status), p.Featured,
		p.Views, p.Claps, p.Author, p.UpdatedAt, p.CreatedAt, p.SeoTitle, p.SeoDescription,
		p.FocusKeyword, p.CanonicalUrl, p.NoIndex,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PostgresRepository) UpdatePost(ctx context.Context, id string, p domain.Post) (*domain.Post, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE posts SET
			title = $2, slug = $3, excerpt = $4, content = $5, category_slug = $6,
			status = $7, featured = $8, views = $9, claps = $10, author = $11,
			updated_at = $12, seo_title = $13, seo_description = $14,
			focus_keyword = $15, canonical_url = $16, no_index = $17
		WHERE id = $1
	`,
		id, p.Title, p.Slug, p.Excerpt, p.Content, p.CategorySlug,
		string(p.Status), p.Featured, p.Views, p.Claps, p.Author,
		p.UpdatedAt, p.SeoTitle, p.SeoDescription,
		p.FocusKeyword, p.CanonicalUrl, p.NoIndex,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	p.ID = id
	return &p, nil
}

func (r *PostgresRepository) DeletePost(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) IncrementCounter(ctx context.Context, slug string, field string) (*domain.Post, error) {
	query := "UPDATE posts SET views = views + 1 WHERE slug = $1"
	if field == "claps" {
		query = "UPDATE posts SET claps = claps + 1 WHERE slug = $1"
	}
	tag, err := r.pool.Exec(ctx, query, slug)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetPostBySlug(ctx, slug)
}

func (r *PostgresRepository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.pool.Query(ctx, "SELECT id, name, slug, parent_slug, description FROM categories ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentSlug, &c.Description); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []domain.Category{}
	}
	return out, nil
}

func (r *PostgresRepository) CreateCategory(ctx context.Context, c domain.Category) (*domain.Category, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, parent_slug, description)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, c.ID, c.Name, c.Slug, c.ParentSlug, c.Description)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PostgresRepository) DeleteCategory(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListCommentsBySlug(ctx context.Context, slug string) ([]domain.CommentItem, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if slug == "" {
		rows, err = r.pool.Query(ctx, "SELECT id, post_slug, author, body, created_at FROM comments ORDER BY created_at DESC")
	} else {
		rows, err = r.pool.Query(ctx, "SELECT id, post_slug, author, body, created_at FROM comments WHERE post_slug = $1 ORDER BY created_at DESC", slug)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CommentItem
	for rows.Next() {
		var c domain.CommentItem
		if err := rows.Scan(&c.ID, &c.PostSlug, &c.Author, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []domain.CommentItem{}
	}
	return out, nil
}

func (r *PostgresRepository) CreateComment(ctx context.Context, c domain.CommentItem) (*domain.CommentItem, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO comments (id, post_slug, author, body, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, c.ID, c.PostSlug, c.Author, c.Body, c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PostgresRepository) GetSettings(ctx context.Context) (*domain.SiteSettings, error) {
	var s domain.SiteSettings
	err := r.pool.QueryRow(ctx, `
		SELECT site_name, tagline, posts_per_page, maintenance_mode, allow_comments,
		       default_seo_description, site_url, twitter_handle
		FROM site_settings WHERE id = 1
	`).Scan(
		&s.SiteName, &s.Tagline, &s.PostsPerPage, &s.MaintenanceMode, &s.AllowComments,
		&s.DefaultSeoDescription, &s.SiteURL, &s.TwitterHandle,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		seed := defaultSeedStore().Settings
		return &seed, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PostgresRepository) UpdateSettings(ctx context.Context, s domain.SiteSettings) (*domain.SiteSettings, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO site_settings (
			id, site_name, tagline, posts_per_page, maintenance_mode, allow_comments,
			default_seo_description, site_url, twitter_handle
		) VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			site_name = EXCLUDED.site_name,
			tagline = EXCLUDED.tagline,
			posts_per_page = EXCLUDED.posts_per_page,
			maintenance_mode = EXCLUDED.maintenance_mode,
			allow_comments = EXCLUDED.allow_comments,
			default_seo_description = EXCLUDED.default_seo_description,
			site_url = EXCLUDED.site_url,
			twitter_handle = EXCLUDED.twitter_handle
	`, s.SiteName, s.Tagline, s.PostsPerPage, s.MaintenanceMode, s.AllowComments, s.DefaultSeoDescription, s.SiteURL, s.TwitterHandle)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PostgresRepository) ListMedia(ctx context.Context) ([]domain.MediaItem, error) {
	rows, err := r.pool.Query(ctx, "SELECT id, filename, url, size_bytes, mime_type, alt, uploaded_at FROM media ORDER BY uploaded_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.MediaItem
	for rows.Next() {
		var m domain.MediaItem
		if err := rows.Scan(&m.ID, &m.Filename, &m.URL, &m.SizeBytes, &m.MimeType, &m.Alt, &m.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []domain.MediaItem{}
	}
	return out, nil
}

func (r *PostgresRepository) CreateMedia(ctx context.Context, m domain.MediaItem) (*domain.MediaItem, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO media (id, filename, url, size_bytes, mime_type, alt, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING
	`, m.ID, m.Filename, m.URL, m.SizeBytes, m.MimeType, m.Alt, m.UploadedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PostgresRepository) DeleteMedia(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM media WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListAuditLogs(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, "SELECT id, action, target, actor, timestamp, meta FROM audit_logs ORDER BY timestamp DESC LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		if err := rows.Scan(&l.ID, &l.Action, &l.Target, &l.Actor, &l.Timestamp, &l.Meta); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if out == nil {
		out = []domain.AuditLog{}
	}
	return out, nil
}

func (r *PostgresRepository) AppendAuditLog(ctx context.Context, l domain.AuditLog) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, action, target, actor, timestamp, meta)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`, l.ID, l.Action, l.Target, l.Actor, l.Timestamp, l.Meta)
	return err
}
