package handler

import (
	"strconv"
	"time"

	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) HealthCheck(c *fiber.Ctx) error {
	infra := map[string]string{}
	if h.healthPing != nil {
		infra = h.healthPing(c.UserContext())
	}
	return c.JSON(fiber.Map{
		"status":    "ok",
		"service":   "exam-blog-api",
		"framework": "gofiber/v2",
		"infra":     infra,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) GetContentSnapshot(c *fiber.Ctx) error {
	includeDrafts := c.Query("draft") == "1" || c.Query("draft") == "true" || c.Query("all") == "1"
	slug := c.Query("slug")

	if slug != "" {
		post, err := h.svc.GetPostBySlug(c.UserContext(), slug, includeDrafts)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "post not found"})
		}
		settings, _ := h.svc.GetSettings(c.UserContext())
		comments, _ := h.svc.ListCommentsBySlug(c.UserContext(), slug)
		return c.JSON(fiber.Map{
			"post":     post,
			"settings": settings,
			"comments": comments,
		})
	}

	snapshot, err := h.svc.GetContentSnapshot(c.UserContext(), true)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if !includeDrafts {
		var published []domain.Post
		for _, p := range snapshot.Posts {
			if p.Status == domain.StatusPublished {
				published = append(published, p)
			}
		}
		snapshot.Posts = published
	}
	return c.JSON(snapshot)
}

func (h *Handler) HandleContentAction(c *fiber.Ctx) error {
	var body struct {
		Action   string `json:"action"`
		Slug     string `json:"slug"`
		PostSlug string `json:"postSlug"`
		Author   string `json:"author"`
		Body     string `json:"body"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	switch body.Action {
	case "increment_view":
		post, err := h.svc.IncrementView(c.UserContext(), body.Slug)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true, "views": post.Views})

	case "clap":
		post, err := h.svc.IncrementClap(c.UserContext(), body.Slug)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true, "claps": post.Claps})

	case "add_comment":
		targetSlug := body.PostSlug
		if targetSlug == "" {
			targetSlug = body.Slug
		}
		cmt, err := h.svc.CreateComment(c.UserContext(), domain.CommentItem{
			PostSlug: targetSlug,
			Author:   body.Author,
			Body:     body.Body,
		})
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true, "comment": cmt})

	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported action"})
	}
}

func (h *Handler) GetDashboardStats(c *fiber.Ctx) error {
	stats, err := h.svc.GetDashboardStats(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

func (h *Handler) GetSettings(c *fiber.Ctx) error {
	settings, err := h.svc.GetSettings(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"settings": settings})
}

func (h *Handler) UpdateSettings(c *fiber.Ctx) error {
	var input domain.SiteSettings
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	updated, err := h.svc.UpdateSettings(c.UserContext(), input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "settings": updated})
}

func (h *Handler) ListMedia(c *fiber.Ctx) error {
	media, err := h.svc.ListMedia(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"media": media})
}

func (h *Handler) CreateMedia(c *fiber.Ctx) error {
	var input domain.MediaItem
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	item, err := h.svc.CreateMedia(c.UserContext(), input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true, "media": item})
}

func (h *Handler) DeleteMedia(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.DeleteMedia(c.UserContext(), id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) ListAuditLogs(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "30"))
	logs, err := h.svc.ListAuditLogs(c.UserContext(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"auditLogs": logs})
}
