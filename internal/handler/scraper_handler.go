package handler

import (
	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/firstthanakorn17/exam-blog-api/internal/scraper"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) ListScrapeSources(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"sources": h.scraperSvc.ListSources(),
	})
}

func (h *Handler) CreateScrapeSource(c *fiber.Ctx) error {
	var input domain.ScrapeSource
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if input.Name == "" || input.PageURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name and pageUrl are required"})
	}
	created := h.scraperSvc.CreateSource(input)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"ok":     true,
		"source": created,
	})
}

func (h *Handler) DeleteScrapeSource(c *fiber.Ctx) error {
	id := c.Params("id")
	if !h.scraperSvc.DeleteSource(id) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "source not found"})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) ListScrapedItems(c *fiber.Ctx) error {
	status := c.Query("status")
	items := h.scraperSvc.ListItems(status)
	return c.JSON(fiber.Map{
		"items": items,
		"total": len(items),
	})
}

func (h *Handler) RunScraperNow(c *fiber.Ctx) error {
	var body struct {
		SourceID string `json:"sourceId"`
	}
	_ = c.BodyParser(&body)

	added, err := h.scraperSvc.RunScrape(c.UserContext(), body.SourceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"ok":         true,
		"newlyAdded": len(added),
		"items":      h.scraperSvc.ListItems(""),
	})
}

func (h *Handler) IngestScraperWebhook(c *fiber.Ctx) error {
	var body struct {
		Posts []scraper.RawFBPost `json:"posts"`
	}
	if err := c.BodyParser(&body); err != nil || len(body.Posts) == 0 {
		var arr []scraper.RawFBPost
		if errArr := c.BodyParser(&arr); errArr == nil && len(arr) > 0 {
			body.Posts = arr
		} else {
			var single scraper.RawFBPost
			if errSingle := c.BodyParser(&single); errSingle == nil && single.Text != "" {
				body.Posts = []scraper.RawFBPost{single}
			} else {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid webhook payload"})
			}
		}
	}

	added, err := h.scraperSvc.IngestWebhook(c.UserContext(), body.Posts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"ok":         true,
		"newlyAdded": len(added),
		"items":      added,
	})
}

func (h *Handler) ApproveScrapedItem(c *fiber.Ctx) error {
	id := c.Params("id")
	post, item, err := h.scraperSvc.ApproveItem(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"ok":   true,
		"post": post,
		"item": item,
	})
}

func (h *Handler) RejectScrapedItem(c *fiber.Ctx) error {
	id := c.Params("id")
	item, err := h.scraperSvc.RejectItem(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"ok":   true,
		"item": item,
	})
}
