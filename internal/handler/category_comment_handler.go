package handler

import (
	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) ListCategories(c *fiber.Ctx) error {
	categories, err := h.svc.ListCategories(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"categories": categories,
	})
}

func (h *Handler) CreateCategory(c *fiber.Ctx) error {
	var input domain.Category
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	cat, err := h.svc.CreateCategory(c.UserContext(), input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"ok":       true,
		"category": cat,
	})
}

func (h *Handler) DeleteCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.DeleteCategory(c.UserContext(), id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) ListComments(c *fiber.Ctx) error {
	slug := c.Query("slug")
	comments, err := h.svc.ListCommentsBySlug(c.UserContext(), slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"comments": comments,
	})
}

func (h *Handler) CreateComment(c *fiber.Ctx) error {
	var input domain.CommentItem
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	comment, err := h.svc.CreateComment(c.UserContext(), input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"ok":      true,
		"comment": comment,
	})
}
