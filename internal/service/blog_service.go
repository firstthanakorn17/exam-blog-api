package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/firstthanakorn17/exam-blog-api/internal/webhook"
)

var (
	ErrValidation = errors.New("validation failed")
)

type blogService struct {
	repo     domain.BlogRepository
	notifier *webhook.Notifier
}

func NewBlogService(repo domain.BlogRepository, notifier *webhook.Notifier) domain.BlogService {
	return &blogService{
		repo:     repo,
		notifier: notifier,
	}
}

func (s *blogService) GetContentSnapshot(ctx context.Context, includeDrafts bool) (*domain.DatabaseStore, error) {
	store, err := s.repo.GetSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	if !includeDrafts {
		var published []domain.Post
		for _, p := range store.Posts {
			if p.Status == domain.StatusPublished {
				published = append(published, p)
			}
		}
		store.Posts = published
	}
	return store, nil
}

func (s *blogService) GetDashboardStats(ctx context.Context) (*domain.DashboardStats, error) {
	store, err := s.repo.GetSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	stats := &domain.DashboardStats{
		TotalPosts:      len(store.Posts),
		TotalComments:   len(store.Comments),
		TotalCategories: len(store.Categories),
	}
	for _, p := range store.Posts {
		switch p.Status {
		case domain.StatusPublished:
			stats.PublishedPosts++
		case domain.StatusDraft:
			stats.DraftPosts++
		case domain.StatusArchived:
			stats.ArchivedPosts++
		}
		stats.TotalViews += p.Views
		stats.TotalClaps += p.Claps
	}
	return stats, nil
}

func (s *blogService) ListPosts(ctx context.Context, filter domain.PostFilter) ([]domain.Post, error) {
	return s.repo.ListPosts(ctx, filter)
}

func (s *blogService) GetPostByID(ctx context.Context, id string) (*domain.Post, error) {
	return s.repo.GetPostByID(ctx, id)
}

func (s *blogService) GetPostBySlug(ctx context.Context, slug string, includeDraft bool) (*domain.Post, error) {
	post, err := s.repo.GetPostBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !includeDraft && post.Status != domain.StatusPublished {
		return nil, errors.New("post is not published")
	}
	return post, nil
}

func (s *blogService) CreatePost(ctx context.Context, input domain.Post) (*domain.Post, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if input.ID == "" {
		input.ID = fmt.Sprintf("post-%d", time.Now().UnixMilli())
	}
	if strings.TrimSpace(input.Slug) == "" {
		input.Slug = slugify(input.Title)
	}
	if input.Status == "" {
		input.Status = domain.StatusDraft
	}
	if input.Author == "" {
		input.Author = "Admin First"
	}
	if input.CategorySlug == "" {
		input.CategorySlug = "engineering"
	}
	input.CreatedAt = now
	input.UpdatedAt = now

	created, err := s.repo.CreatePost(ctx, input)
	if err != nil {
		return nil, err
	}

	_ = s.repo.AppendAuditLog(ctx, domain.AuditLog{
		Action: "CREATE_POST",
		Target: created.Title,
		Actor:  created.Author,
		Meta:   string(created.Status),
	})
	s.notifier.NotifyTags("posts", "post-"+created.Slug)
	return created, nil
}

func (s *blogService) UpdatePost(ctx context.Context, id string, input domain.Post) (*domain.Post, error) {
	existing, err := s.repo.GetPostByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.Title) == "" {
		input.Title = existing.Title
	}
	if strings.TrimSpace(input.Slug) == "" {
		input.Slug = existing.Slug
	}
	if input.Status == "" {
		input.Status = existing.Status
	}
	if input.CategorySlug == "" {
		input.CategorySlug = existing.CategorySlug
	}
	if input.Author == "" {
		input.Author = existing.Author
	}
	if input.Views == 0 {
		input.Views = existing.Views
	}
	if input.Claps == 0 {
		input.Claps = existing.Claps
	}
	input.CreatedAt = existing.CreatedAt
	input.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	updated, err := s.repo.UpdatePost(ctx, id, input)
	if err != nil {
		return nil, err
	}

	_ = s.repo.AppendAuditLog(ctx, domain.AuditLog{
		Action: "UPDATE_POST",
		Target: updated.Title,
		Actor:  updated.Author,
		Meta:   string(updated.Status),
	})
	s.notifier.NotifyTags("posts", "post-"+updated.Slug)
	return updated, nil
}

func (s *blogService) TogglePostStatus(ctx context.Context, id string) (*domain.Post, error) {
	existing, err := s.repo.GetPostByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Status == domain.StatusPublished {
		existing.Status = domain.StatusDraft
	} else {
		existing.Status = domain.StatusPublished
	}
	existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	updated, err := s.repo.UpdatePost(ctx, id, *existing)
	if err != nil {
		return nil, err
	}

	_ = s.repo.AppendAuditLog(ctx, domain.AuditLog{
		Action: "TOGGLE_STATUS",
		Target: updated.Title,
		Actor:  "Admin First",
		Meta:   string(updated.Status),
	})
	s.notifier.NotifyTags("posts", "post-"+updated.Slug)
	return updated, nil
}

func (s *blogService) DeletePost(ctx context.Context, id string) error {
	existing, err := s.repo.GetPostByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeletePost(ctx, id); err != nil {
		return err
	}

	_ = s.repo.AppendAuditLog(ctx, domain.AuditLog{
		Action: "DELETE_POST",
		Target: existing.Title,
		Actor:  "Admin First",
	})
	s.notifier.NotifyTags("posts", "post-"+existing.Slug)
	return nil
}

func (s *blogService) IncrementView(ctx context.Context, slug string) (*domain.Post, error) {
	return s.repo.IncrementCounter(ctx, slug, "views")
}

func (s *blogService) IncrementClap(ctx context.Context, slug string) (*domain.Post, error) {
	post, err := s.repo.IncrementCounter(ctx, slug, "claps")
	if err != nil {
		return nil, err
	}
	s.notifier.NotifyTags("posts", "post-"+slug)
	return post, nil
}

func (s *blogService) ListCategories(ctx context.Context) ([]domain.Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *blogService) CreateCategory(ctx context.Context, input domain.Category) (*domain.Category, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("%w: category name is required", ErrValidation)
	}
	if input.ID == "" {
		input.ID = fmt.Sprintf("cat-%d", time.Now().UnixMilli())
	}
	baseSlug := slugify(input.Slug)
	if baseSlug == "" {
		baseSlug = slugify(input.Name)
	}
	if input.ParentSlug != nil && strings.TrimSpace(*input.ParentSlug) != "" {
		parent := strings.TrimSpace(*input.ParentSlug)
		if !strings.HasPrefix(baseSlug, parent+"/") {
			input.Slug = parent + "/" + baseSlug
		} else {
			input.Slug = baseSlug
		}
	} else {
		input.ParentSlug = nil
		input.Slug = baseSlug
	}

	created, err := s.repo.CreateCategory(ctx, input)
	if err != nil {
		return nil, err
	}

	_ = s.repo.AppendAuditLog(ctx, domain.AuditLog{
		Action: "CREATE_CATEGORY",
		Target: created.Name,
		Actor:  "Admin First",
		Meta:   created.Slug,
	})
	s.notifier.NotifyTags("categories")
	return created, nil
}

func (s *blogService) DeleteCategory(ctx context.Context, id string) error {
	if err := s.repo.DeleteCategory(ctx, id); err != nil {
		return err
	}
	_ = s.repo.AppendAuditLog(ctx, domain.AuditLog{
		Action: "DELETE_CATEGORY",
		Target: id,
		Actor:  "Admin First",
	})
	s.notifier.NotifyTags("categories")
	return nil
}

func (s *blogService) ListCommentsBySlug(ctx context.Context, slug string) ([]domain.CommentItem, error) {
	return s.repo.ListCommentsBySlug(ctx, slug)
}

func (s *blogService) CreateComment(ctx context.Context, input domain.CommentItem) (*domain.CommentItem, error) {
	if strings.TrimSpace(input.PostSlug) == "" || strings.TrimSpace(input.Author) == "" || strings.TrimSpace(input.Body) == "" {
		return nil, fmt.Errorf("%w: postSlug, author, and body are required", ErrValidation)
	}
	input.ID = fmt.Sprintf("cmt-%d", time.Now().UnixMilli())
	input.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	created, err := s.repo.CreateComment(ctx, input)
	if err != nil {
		return nil, err
	}
	s.notifier.NotifyTags("comments", "comments-"+input.PostSlug)
	return created, nil
}

func (s *blogService) GetSettings(ctx context.Context) (*domain.SiteSettings, error) {
	return s.repo.GetSettings(ctx)
}

func (s *blogService) UpdateSettings(ctx context.Context, input domain.SiteSettings) (*domain.SiteSettings, error) {
	updated, err := s.repo.UpdateSettings(ctx, input)
	if err != nil {
		return nil, err
	}
	_ = s.repo.AppendAuditLog(ctx, domain.AuditLog{
		Action: "UPDATE_SETTINGS",
		Target: updated.SiteName,
		Actor:  "Admin First",
	})
	s.notifier.NotifyTags("settings")
	return updated, nil
}

func (s *blogService) ListMedia(ctx context.Context) ([]domain.MediaItem, error) {
	return s.repo.ListMedia(ctx)
}

func (s *blogService) CreateMedia(ctx context.Context, input domain.MediaItem) (*domain.MediaItem, error) {
	if input.ID == "" {
		input.ID = fmt.Sprintf("med-%d", time.Now().UnixMilli())
	}
	if input.UploadedAt == "" {
		input.UploadedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return s.repo.CreateMedia(ctx, input)
}

func (s *blogService) DeleteMedia(ctx context.Context, id string) error {
	return s.repo.DeleteMedia(ctx, id)
}

func (s *blogService) ListAuditLogs(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	return s.repo.ListAuditLogs(ctx, limit)
}

var nonSlugRegex = regexp.MustCompile(`[^a-z0-9ก-๙\-/]+`)

func slugify(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	s = strings.ReplaceAll(s, " ", "-")
	s = nonSlugRegex.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	if s == "" {
		return fmt.Sprintf("item-%d", time.Now().Unix())
	}
	return s
}
