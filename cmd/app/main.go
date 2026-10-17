package main

import (
	"context"
	"go-pet-shop/internal/config"
	"go-pet-shop/internal/handlers"
	"go-pet-shop/internal/handlers/product"
	userHandler "go-pet-shop/internal/handlers/user"
	"go-pet-shop/internal/lib/logger"
	"go-pet-shop/internal/storage/postgres"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.MustLoad()

	// Settings logger
	log := logger.SetupLogger(cfg.Env)
	log.Info("starting the project...", slog.String("env", cfg.Env))

	// Settings and started database
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseTimeout)
	defer cancel()

	storage, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to init storage", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Error("failed to close storage", slog.String("error", err.Error()))
		}
		log.Info("storage closed")
	}()

	// Init router
	router := chi.NewRouter()

	// Middlewares
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)
	router.Use(logger.CustomLogger(log))

	// Handlers
	productHandler := product.New(log, storage)
	usersHandler := userHandler.New(log, storage)
	router.Get("/health", handlers.StatusHandler)
	router.Post("/users", usersHandler.CreateUser)
	router.Get("/users", usersHandler.GetAllUsers)
	router.Get("/users/{email}", usersHandler.GetUserByEmail)
	router.Get("/products", productHandler.GetAllProducts)
	router.Get("/products/{id}", productHandler.GetProductByID)
	router.Post("/products", productHandler.CreateProduct)
	router.Delete("/products/{id}", productHandler.DeleteProduct)
	router.Put("/products/{id}", productHandler.UpdateProduct)

	// Settings and started server
	srv := &http.Server{
		Addr:         cfg.Address,
		Handler:      router,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Запуск сервера в горутине для graceful shutdown
	serverErrors := make(chan error, 1)
	go func() {
		log.Info("Starting server on", slog.String("address", cfg.Address))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Error("Server error", slog.String("error", err.Error()))
		return
	case <-stop:
	}

	log.Info("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server shutdown error", slog.String("error", err.Error()))
		if closeErr := srv.Close(); closeErr != nil {
			log.Error("Server close error", slog.String("error", closeErr.Error()))
		}
		return
	}

	log.Info("Server stopped gracefully")
}
