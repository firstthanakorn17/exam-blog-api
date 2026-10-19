package handler

import (
	"context"

	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/firstthanakorn17/exam-blog-api/internal/scraper"
	"github.com/gofiber/fiber/v2"
)

type HealthPinger func(ctx context.Context) map[string]string

type Handler struct {
	svc        domain.BlogService
	scraperSvc *scraper.Service
	healthPing HealthPinger
}

func NewHandler(svc domain.BlogService, scraperSvc *scraper.Service, healthPing HealthPinger) *Handler {
	return &Handler{
		svc:        svc,
		scraperSvc: scraperSvc,
		healthPing: healthPing,
	}
}

func RegisterRoutes(app *fiber.App, h *Handler) {
	app.Get("/health", h.HealthCheck)

	v1 := app.Group("/api/v1")

	// Snapshot & Dashboard Stats (used by Next.js Console & Reader Interface)
	v1.Get("/content", h.GetContentSnapshot)
	v1.Post("/content", h.HandleContentAction)
	v1.Get("/stats", h.GetDashboardStats)

	// Posts REST API
	v1.Get("/posts", h.ListPosts)
	v1.Get("/posts/:idOrSlug", h.GetPost)
	v1.Post("/posts", h.CreatePost)
	v1.Put("/posts/:id", h.UpdatePost)
	v1.Patch("/posts/:id/toggle-status", h.TogglePostStatus)
	v1.Delete("/posts/:id", h.DeletePost)
	v1.Post("/posts/:slug/view", h.IncrementView)
	v1.Post("/posts/:slug/clap", h.IncrementClap)

	// Categories REST API
	v1.Get("/categories", h.ListCategories)
	v1.Post("/categories", h.CreateCategory)
	v1.Delete("/categories/:id", h.DeleteCategory)

	// Comments REST API
	v1.Get("/comments", h.ListComments)
	v1.Post("/comments", h.CreateComment)

	// Site Settings, Media & Audit Logs
	v1.Get("/settings", h.GetSettings)
	v1.Put("/settings", h.UpdateSettings)
	v1.Get("/media", h.ListMedia)
	v1.Post("/media", h.CreateMedia)
	v1.Delete("/media/:id", h.DeleteMedia)
	v1.Get("/audit-logs", h.ListAuditLogs)
	v1.Post("/audit-logs", h.CreateAuditLog)

	// Facebook Exam Scraper Radar REST API
	v1.Get("/scraper/sources", h.ListScrapeSources)
	v1.Post("/scraper/sources", h.CreateScrapeSource)
	v1.Delete("/scraper/sources/:id", h.DeleteScrapeSource)
	v1.Get("/scraper/items", h.ListScrapedItems)
	v1.Post("/scraper/run", h.RunScraperNow)
	v1.Post("/scraper/webhook", h.IngestScraperWebhook)
	v1.Post("/scraper/items/:id/approve", h.ApproveScrapedItem)
	v1.Post("/scraper/items/:id/reject", h.RejectScrapedItem)
}
