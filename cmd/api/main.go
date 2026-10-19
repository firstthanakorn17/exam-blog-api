package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/firstthanakorn17/exam-blog-api/internal/cache"
	"github.com/firstthanakorn17/exam-blog-api/internal/config"
	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
	"github.com/firstthanakorn17/exam-blog-api/internal/handler"
	"github.com/firstthanakorn17/exam-blog-api/internal/middleware"
	"github.com/firstthanakorn17/exam-blog-api/internal/repository"
	"github.com/firstthanakorn17/exam-blog-api/internal/scraper"
	"github.com/firstthanakorn17/exam-blog-api/internal/service"
	"github.com/firstthanakorn17/exam-blog-api/internal/webhook"
	"github.com/gofiber/fiber/v2"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var baseRepo domain.BlogRepository
	var pgRepo *repository.PostgresRepository

	pg, err := repository.NewPostgresRepository(ctx, cfg.DatabaseURL, cfg.DataFile)
	if err != nil {
		log.Printf("[warn] PostgreSQL unavailable (%v), falling back to JSONRepository (%s)", err, cfg.DataFile)
		jsonRepo, jsonErr := repository.NewJSONRepository(cfg.DataFile)
		if jsonErr != nil {
			log.Fatalf("failed to initialize fallback repository: %v", jsonErr)
		}
		baseRepo = jsonRepo
	} else {
		log.Println("[init] Connected to PostgreSQL and verified schema + seed data")
		pgRepo = pg
		baseRepo = pg
		defer pg.Close()
	}

	cachedRepo := cache.NewCachedRepository(baseRepo, cfg.RedisAddr, cfg.RedisPassword)
	defer func() { _ = cachedRepo.Close() }()

	if err := cachedRepo.Ping(ctx); err != nil {
		log.Printf("[warn] Redis ping warning (%s): %v", cfg.RedisAddr, err)
	} else {
		log.Printf("[init] Connected to Redis cache (%s)", cfg.RedisAddr)
	}

	notifier := webhook.NewNotifier(cfg.ReaderWebhookURL, cfg.WebhookSecret)
	svc := service.NewBlogService(cachedRepo, notifier)
	scraperSvc := scraper.NewService(svc)

	h := handler.NewHandler(svc, scraperSvc, func(pingCtx context.Context) map[string]string {
		dbStatus := "fallback:json"
		if pgRepo != nil {
			if err := pgRepo.Ping(pingCtx); err == nil {
				dbStatus = "postgres:connected"
			} else {
				dbStatus = "postgres:disconnected"
			}
		}
		redisStatus := "redis:connected"
		if err := cachedRepo.Ping(pingCtx); err != nil {
			redisStatus = "redis:disconnected"
		}
		return map[string]string{
			"database": dbStatus,
			"cache":    redisStatus,
		}
	})

	app := fiber.New(fiber.Config{
		AppName:               "exam-blog-api (Go Fiber v2 + Postgres + Redis)",
		DisableStartupMessage: false,
	})

	middleware.Setup(app, cfg)
	handler.RegisterRoutes(app, h)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down exam-blog-api server...")
		_ = app.Shutdown()
	}()

	log.Printf("Starting exam-blog-api on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
