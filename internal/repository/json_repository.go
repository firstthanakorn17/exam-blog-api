package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
)

var (
	ErrNotFound = errors.New("resource not found")
)

type JSONRepository struct {
	mu       sync.RWMutex
	filePath string
}

func NewJSONRepository(filePath string) (*JSONRepository, error) {
	repo := &JSONRepository{
		filePath: filePath,
	}
	if err := repo.ensureFile(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *JSONRepository) ensureFile() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := os.Stat(r.filePath); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(r.filePath), 0o755); err != nil {
		return err
	}

	seed := defaultSeedStore()
	return r.writeStoreLocked(&seed)
}

func (r *JSONRepository) readStoreLocked() (*domain.DatabaseStore, error) {
	raw, err := os.ReadFile(r.filePath)
	if err != nil {
		return nil, err
	}
	var store domain.DatabaseStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return nil, err
	}
	if store.Comments == nil {
		store.Comments = defaultComments()
	}
	return &store, nil
}

func (r *JSONRepository) writeStoreLocked(store *domain.DatabaseStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, data, 0o644)
}

func (r *JSONRepository) GetSnapshot(ctx context.Context) (*domain.DatabaseStore, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readStoreLocked()
}

func (r *JSONRepository) ListPosts(ctx context.Context, filter domain.PostFilter) ([]domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(filter.Query))
	var out []domain.Post
	for _, p := range store.Posts {
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
	return out, nil
}

func (r *JSONRepository) GetPostByID(ctx context.Context, id string) (*domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	for _, p := range store.Posts {
		if p.ID == id {
			cp := p
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (r *JSONRepository) GetPostBySlug(ctx context.Context, slug string) (*domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	for _, p := range store.Posts {
		if p.Slug == slug {
			cp := p
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (r *JSONRepository) CreatePost(ctx context.Context, post domain.Post) (*domain.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	store.Posts = append([]domain.Post{post}, store.Posts...)
	if err := r.writeStoreLocked(store); err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *JSONRepository) UpdatePost(ctx context.Context, id string, updated domain.Post) (*domain.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	for i, p := range store.Posts {
		if p.ID == id {
			updated.ID = p.ID
			if updated.CreatedAt == "" {
				updated.CreatedAt = p.CreatedAt
			}
			store.Posts[i] = updated
			if err := r.writeStoreLocked(store); err != nil {
				return nil, err
			}
			return &updated, nil
		}
	}
	return nil, ErrNotFound
}

func (r *JSONRepository) DeletePost(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return err
	}
	idx := -1
	for i, p := range store.Posts {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrNotFound
	}
	store.Posts = append(store.Posts[:idx], store.Posts[idx+1:]...)
	return r.writeStoreLocked(store)
}

func (r *JSONRepository) IncrementCounter(ctx context.Context, slug string, field string) (*domain.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	for i, p := range store.Posts {
		if p.Slug == slug {
			if field == "claps" {
				if store.Posts[i].Claps == 0 {
					store.Posts[i].Claps = 12
				}
				store.Posts[i].Claps++
			} else {
				store.Posts[i].Views++
			}
			if err := r.writeStoreLocked(store); err != nil {
				return nil, err
			}
			cp := store.Posts[i]
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (r *JSONRepository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	return store.Categories, nil
}

func (r *JSONRepository) CreateCategory(ctx context.Context, category domain.Category) (*domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	store.Categories = append(store.Categories, category)
	if err := r.writeStoreLocked(store); err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *JSONRepository) DeleteCategory(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return err
	}
	idx := -1
	for i, c := range store.Categories {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrNotFound
	}
	store.Categories = append(store.Categories[:idx], store.Categories[idx+1:]...)
	return r.writeStoreLocked(store)
}

func (r *JSONRepository) ListCommentsBySlug(ctx context.Context, slug string) ([]domain.CommentItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	var out []domain.CommentItem
	for _, c := range store.Comments {
		if slug == "" || c.PostSlug == slug {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *JSONRepository) CreateComment(ctx context.Context, comment domain.CommentItem) (*domain.CommentItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	store.Comments = append([]domain.CommentItem{comment}, store.Comments...)
	if err := r.writeStoreLocked(store); err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *JSONRepository) GetSettings(ctx context.Context) (*domain.SiteSettings, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	return &store.Settings, nil
}

func (r *JSONRepository) UpdateSettings(ctx context.Context, settings domain.SiteSettings) (*domain.SiteSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	store.Settings = settings
	if err := r.writeStoreLocked(store); err != nil {
		return nil, err
	}
	return &store.Settings, nil
}

func (r *JSONRepository) ListMedia(ctx context.Context) ([]domain.MediaItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	return store.Media, nil
}

func (r *JSONRepository) CreateMedia(ctx context.Context, item domain.MediaItem) (*domain.MediaItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	store.Media = append([]domain.MediaItem{item}, store.Media...)
	if err := r.writeStoreLocked(store); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *JSONRepository) DeleteMedia(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return err
	}
	idx := -1
	for i, m := range store.Media {
		if m.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrNotFound
	}
	store.Media = append(store.Media[:idx], store.Media[idx+1:]...)
	return r.writeStoreLocked(store)
}

func (r *JSONRepository) ListAuditLogs(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(store.AuditLogs) > limit {
		return store.AuditLogs[:limit], nil
	}
	return store.AuditLogs, nil
}

func (r *JSONRepository) AppendAuditLog(ctx context.Context, entry domain.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	store, err := r.readStoreLocked()
	if err != nil {
		return err
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("log-%d", time.Now().UnixNano())
	}
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	store.AuditLogs = append([]domain.AuditLog{entry}, store.AuditLogs...)
	if len(store.AuditLogs) > 100 {
		store.AuditLogs = store.AuditLogs[:100]
	}
	return r.writeStoreLocked(store)
}

func defaultComments() []domain.CommentItem {
	return []domain.CommentItem{
		{
			ID:        "cmt-1",
			PostSlug:  "nextjs-16-cache-components-use-cache",
			Author:    "Nattapong S.",
			Body:      "อธิบายเรื่อง use cache กับ updateTag ได้เห็นภาพชัดเจนมากครับ ลองเอาไปปรับใช้กับโปรเจกต์แล้วเร็วขึ้นมาก",
			CreatedAt: "2026-10-16T09:30:00.000Z",
		},
		{
			ID:        "cmt-2",
			PostSlug:  "parallel-and-intercepting-routes-pattern",
			Author:    "Ploypailin K.",
			Body:      "ชอบการแยก @comments กับ @related เป็น Parallel Routes มากค่ะ เวลาโหลดคอมเมนต์ช้าก็ไม่บล็อกเนื้อหาบทความหลัก",
			CreatedAt: "2026-10-16T11:15:00.000Z",
		},
	}
}

func defaultSeedStore() domain.DatabaseStore {
	eng := "engineering"
	return domain.DatabaseStore{
		Posts: []domain.Post{
			{
				ID:           "post-1",
				Title:        "เจาะลึก Next.js 16 Cache Components และ use cache Directive",
				Slug:         "nextjs-16-cache-components-use-cache",
				Excerpt:      "ทำความเข้าใจโมเดลการทำ Caching ใหม่ด้วย 'use cache', cacheLife(), cacheTag() และ updateTag() สำหรับ Read-your-own-writes",
				Content:      "# Next.js 16 Cache Components\n\nใน Next.js 16 ระบบ Caching ถูกยกระดับด้วย `'use cache'` directive ที่กำหนดได้ทั้งระดับ Data, Component และ File พร้อมกับ `cacheTag` และ `updateTag` ใน Server Actions เพื่อให้รองรับทั้งความเร็วระดับ Static และความสดใหม่ของข้อมูลแบบ Read-your-own-writes",
				CategorySlug: "engineering/nextjs",
				Status:       domain.StatusPublished,
				Featured:     true,
				Views:        1840,
				Claps:        42,
				Author:       "Admin First",
				CreatedAt:    "2026-10-10T08:00:00.000Z",
				UpdatedAt:    "2026-10-15T14:20:00.000Z",
				FocusKeyword: "next.js 16",
			},
		},
		Categories: []domain.Category{
			{
				ID:          "cat-1",
				Name:        "Engineering",
				Slug:        "engineering",
				ParentSlug:  nil,
				Description: "บทความด้านวิศวกรรมซอฟต์แวร์และสถาปัตยกรรมระบบ",
			},
			{
				ID:          "cat-2",
				Name:        "Next.js",
				Slug:        "engineering/nextjs",
				ParentSlug:  &eng,
				Description: "เจาะลึก App Router, RSC, Caching และ Turbopack",
			},
		},
		Comments: defaultComments(),
		Media:    []domain.MediaItem{},
		AuditLogs: []domain.AuditLog{
			{
				ID:        "log-init",
				Action:    "GO_API_INIT",
				Target:    "Initialized Go Fiber Clean Architecture Store",
				Actor:     "exam-blog-api",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		},
		Settings: domain.SiteSettings{
			SiteName:              "NextBlog",
			Tagline:               "Modern Engineering & Architecture Journal",
			PostsPerPage:          10,
			MaintenanceMode:       false,
			AllowComments:         true,
			DefaultSeoDescription: "คลังบทความเจาะลึกสถาปัตยกรรมเว็บยุคใหม่ Next.js 16, React 19 และ Go Fiber Backend",
			SiteURL:               "http://localhost:3002",
			TwitterHandle:         "@nextblog",
		},
	}
}
