package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/firstthanakorn17/exam-blog-api/internal/config"
	"github.com/firstthanakorn17/exam-blog-api/internal/handler"
	"github.com/firstthanakorn17/exam-blog-api/internal/middleware"
	"github.com/firstthanakorn17/exam-blog-api/internal/repository"
	"github.com/firstthanakorn17/exam-blog-api/internal/service"
	"github.com/firstthanakorn17/exam-blog-api/internal/webhook"
	"github.com/gofiber/fiber/v2"
)

func main() {
	cfg := config.Load()

	repo, err := repository.NewJSONRepository(cfg.DataFile)
	if err != nil {
		log.Fatalf("failed to initialize repository (%s): %v", cfg.DataFile, err)
	}

	notifier := webhook.NewNotifier(cfg.ReaderWebhookURL, cfg.WebhookSecret)
	svc := service.NewBlogService(repo, notifier)
	h := handler.NewHandler(svc)

	app := fiber.New(fiber.Config{
		AppName:               "exam-blog-api (Go Fiber v2)",
		DisableStartupMessage: false,
	})

	middleware.Setup(app, cfg)
	handler.RegisterRoutes(app, h)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down exam-blog-api server...")
		_ = app.Shutdown()
	}()

	log.Printf("Starting exam-blog-api on :%s (data: %s)", cfg.Port, cfg.DataFile)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
