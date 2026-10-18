package handler

import (
	"strings"

	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) ListPosts(c *fiber.Ctx) error {
	filter := domain.PostFilter{
		Status:       c.Query("status"),
		CategorySlug: c.Query("category"),
		Query:        c.Query("q"),
		IncludeDraft: c.Query("draft") == "1" || c.Query("draft") == "true",
	}

	posts, err := h.svc.ListPosts(c.UserContext(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"posts": posts,
		"total": len(posts),
	})
}

func (h *Handler) GetPost(c *fiber.Ctx) error {
	idOrSlug := c.Params("idOrSlug")
	includeDraft := c.Query("draft") == "1" || c.Query("draft") == "true"

	if strings.HasPrefix(idOrSlug, "post-") {
		post, err := h.svc.GetPostByID(c.UserContext(), idOrSlug)
		if err == nil {
			return c.JSON(fiber.Map{"post": post})
		}
	}

	post, err := h.svc.GetPostBySlug(c.UserContext(), idOrSlug, includeDraft)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "post not found",
		})
	}
	return c.JSON(fiber.Map{"post": post})
}

func (h *Handler) CreatePost(c *fiber.Ctx) error {
	var input domain.Post
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	post, err := h.svc.CreatePost(c.UserContext(), input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"ok":   true,
		"post": post,
	})
}

func (h *Handler) UpdatePost(c *fiber.Ctx) error {
	id := c.Params("id")
	var input domain.Post
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	post, err := h.svc.UpdatePost(c.UserContext(), id, input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"ok":   true,
		"post": post,
	})
}

func (h *Handler) TogglePostStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	post, err := h.svc.TogglePostStatus(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"ok":   true,
		"post": post,
	})
}

func (h *Handler) DeletePost(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.DeletePost(c.UserContext(), id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) IncrementView(c *fiber.Ctx) error {
	slug := c.Params("slug")
	post, err := h.svc.IncrementView(c.UserContext(), slug)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"ok":    true,
		"views": post.Views,
	})
}

func (h *Handler) IncrementClap(c *fiber.Ctx) error {
	slug := c.Params("slug")
	post, err := h.svc.IncrementClap(c.UserContext(), slug)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"ok":    true,
		"claps": post.Claps,
	})
}
