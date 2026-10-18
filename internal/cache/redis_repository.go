package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	keySnapshot   = "examblog:snapshot"
	keyCategories = "examblog:categories"
	keySettings   = "examblog:settings"
	defaultTTL    = 5 * time.Minute
)

type CachedRepository struct {
	next   domain.BlogRepository
	client *redis.Client
}

func NewCachedRepository(next domain.BlogRepository, redisAddr, redisPassword string) *CachedRepository {
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPassword,
		DB:           0,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})
	return &CachedRepository{
		next:   next,
		client: rdb,
	}
}

func (c *CachedRepository) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *CachedRepository) Close() error {
	return c.client.Close()
}

func (c *CachedRepository) invalidate(ctx context.Context, keys ...string) {
	if len(keys) > 0 {
		_ = c.client.Del(ctx, keys...).Err()
	}
}

func (c *CachedRepository) GetSnapshot(ctx context.Context) (*domain.DatabaseStore, error) {
	if raw, err := c.client.Get(ctx, keySnapshot).Bytes(); err == nil {
		var store domain.DatabaseStore
		if json.Unmarshal(raw, &store) == nil {
			return &store, nil
		}
	}

	store, err := c.next.GetSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	if data, err := json.Marshal(store); err == nil {
		_ = c.client.Set(ctx, keySnapshot, data, defaultTTL).Err()
	}
	return store, nil
}

func (c *CachedRepository) ListPosts(ctx context.Context, filter domain.PostFilter) ([]domain.Post, error) {
	return c.next.ListPosts(ctx, filter)
}

func (c *CachedRepository) GetPostByID(ctx context.Context, id string) (*domain.Post, error) {
	return c.next.GetPostByID(ctx, id)
}

func (c *CachedRepository) GetPostBySlug(ctx context.Context, slug string) (*domain.Post, error) {
	cacheKey := fmt.Sprintf("examblog:post:%s", slug)
	if raw, err := c.client.Get(ctx, cacheKey).Bytes(); err == nil {
		var post domain.Post
		if json.Unmarshal(raw, &post) == nil {
			return &post, nil
		}
	}

	post, err := c.next.GetPostBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if data, err := json.Marshal(post); err == nil {
		_ = c.client.Set(ctx, cacheKey, data, defaultTTL).Err()
	}
	return post, nil
}

func (c *CachedRepository) CreatePost(ctx context.Context, post domain.Post) (*domain.Post, error) {
	created, err := c.next.CreatePost(ctx, post)
	if err == nil {
		c.invalidate(ctx, keySnapshot, fmt.Sprintf("examblog:post:%s", created.Slug))
	}
	return created, err
}

func (c *CachedRepository) UpdatePost(ctx context.Context, id string, post domain.Post) (*domain.Post, error) {
	updated, err := c.next.UpdatePost(ctx, id, post)
	if err == nil {
		c.invalidate(ctx, keySnapshot, fmt.Sprintf("examblog:post:%s", updated.Slug))
	}
	return updated, err
}

func (c *CachedRepository) DeletePost(ctx context.Context, id string) error {
	existing, _ := c.next.GetPostByID(ctx, id)
	err := c.next.DeletePost(ctx, id)
	if err == nil {
		if existing != nil {
			c.invalidate(ctx, keySnapshot, fmt.Sprintf("examblog:post:%s", existing.Slug))
		} else {
			c.invalidate(ctx, keySnapshot)
		}
	}
	return err
}

func (c *CachedRepository) IncrementCounter(ctx context.Context, slug string, field string) (*domain.Post, error) {
	post, err := c.next.IncrementCounter(ctx, slug, field)
	if err == nil {
		c.invalidate(ctx, keySnapshot, fmt.Sprintf("examblog:post:%s", slug))
	}
	return post, err
}

func (c *CachedRepository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	if raw, err := c.client.Get(ctx, keyCategories).Bytes(); err == nil {
		var cats []domain.Category
		if json.Unmarshal(raw, &cats) == nil {
			return cats, nil
		}
	}

	cats, err := c.next.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	if data, err := json.Marshal(cats); err == nil {
		_ = c.client.Set(ctx, keyCategories, data, defaultTTL).Err()
	}
	return cats, nil
}

func (c *CachedRepository) CreateCategory(ctx context.Context, category domain.Category) (*domain.Category, error) {
	created, err := c.next.CreateCategory(ctx, category)
	if err == nil {
		c.invalidate(ctx, keySnapshot, keyCategories)
	}
	return created, err
}

func (c *CachedRepository) DeleteCategory(ctx context.Context, id string) error {
	err := c.next.DeleteCategory(ctx, id)
	if err == nil {
		c.invalidate(ctx, keySnapshot, keyCategories)
	}
	return err
}

func (c *CachedRepository) ListCommentsBySlug(ctx context.Context, slug string) ([]domain.CommentItem, error) {
	cacheKey := fmt.Sprintf("examblog:comments:%s", slug)
	if raw, err := c.client.Get(ctx, cacheKey).Bytes(); err == nil {
		var comments []domain.CommentItem
		if json.Unmarshal(raw, &comments) == nil {
			return comments, nil
		}
	}

	comments, err := c.next.ListCommentsBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if data, err := json.Marshal(comments); err == nil {
		_ = c.client.Set(ctx, cacheKey, data, defaultTTL).Err()
	}
	return comments, nil
}

func (c *CachedRepository) CreateComment(ctx context.Context, comment domain.CommentItem) (*domain.CommentItem, error) {
	created, err := c.next.CreateComment(ctx, comment)
	if err == nil {
		c.invalidate(ctx, keySnapshot, "examblog:comments:", fmt.Sprintf("examblog:comments:%s", comment.PostSlug))
	}
	return created, err
}

func (c *CachedRepository) GetSettings(ctx context.Context) (*domain.SiteSettings, error) {
	if raw, err := c.client.Get(ctx, keySettings).Bytes(); err == nil {
		var s domain.SiteSettings
		if json.Unmarshal(raw, &s) == nil {
			return &s, nil
		}
	}

	s, err := c.next.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if data, err := json.Marshal(s); err == nil {
		_ = c.client.Set(ctx, keySettings, data, defaultTTL).Err()
	}
	return s, nil
}

func (c *CachedRepository) UpdateSettings(ctx context.Context, settings domain.SiteSettings) (*domain.SiteSettings, error) {
	updated, err := c.next.UpdateSettings(ctx, settings)
	if err == nil {
		c.invalidate(ctx, keySnapshot, keySettings)
	}
	return updated, err
}

func (c *CachedRepository) ListMedia(ctx context.Context) ([]domain.MediaItem, error) {
	return c.next.ListMedia(ctx)
}

func (c *CachedRepository) CreateMedia(ctx context.Context, item domain.MediaItem) (*domain.MediaItem, error) {
	created, err := c.next.CreateMedia(ctx, item)
	if err == nil {
		c.invalidate(ctx, keySnapshot)
	}
	return created, err
}

func (c *CachedRepository) DeleteMedia(ctx context.Context, id string) error {
	err := c.next.DeleteMedia(ctx, id)
	if err == nil {
		c.invalidate(ctx, keySnapshot)
	}
	return err
}

func (c *CachedRepository) ListAuditLogs(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	return c.next.ListAuditLogs(ctx, limit)
}

func (c *CachedRepository) AppendAuditLog(ctx context.Context, logEntry domain.AuditLog) error {
	err := c.next.AppendAuditLog(ctx, logEntry)
	if err == nil {
		c.invalidate(ctx, keySnapshot)
	}
	return err
}
