package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"openbpl/internal/config"
	"openbpl/internal/database"
	"openbpl/internal/handlers"
	"openbpl/internal/middleware"
	"openbpl/pkg/models"
)

func main() {
	cfg := config.Load()

	log.Printf("🚀 Starting OpenBPL server...")
	log.Printf("📋 Environment: %s", cfg.Environment)
	log.Printf("🌐 Port: %s", cfg.Port)

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("❌ Failed to connect to database:", err)
	}
	defer db.Close()

	userRepo := models.NewUserRepository(db.DB)
	threatRepo := models.NewThreatRepository(db.DB)

	dbHandlers := handlers.NewDatabaseHandlers(userRepo, threatRepo)

	mux := http.NewServeMux()

	setupRoutes(mux, dbHandlers)

	server := &http.Server{
		Addr:         cfg.Port,
		Handler:      middleware.Chain(mux, middleware.Logger, middleware.CORS, middleware.Recovery),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🚀 Server starting on %s", cfg.Port)
		log.Printf("📖 Available endpoints:")
		log.Printf("   GET  %s/health - Health check", cfg.Port)
		log.Printf("   GET  %s/api/v1/status - Status info", cfg.Port)
		log.Printf("   GET  %s/api/v1/users - List users", cfg.Port)
		log.Printf("   GET  %s/api/v1/users/{id} - Get user by ID", cfg.Port)
		log.Printf("   GET  %s/api/v1/threats - List threats", cfg.Port)
		log.Printf("   GET  %s/api/v1/threats/{id} - Get threat by ID", cfg.Port)
		log.Printf("   GET  %s/ - Home page", cfg.Port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("❌ Server failed to start:", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("❌ Server forced to shutdown:", err)
	}

	log.Println("✅ Server stopped gracefully")
}

func setupRoutes(mux *http.ServeMux, dbHandlers *handlers.DatabaseHandlers) {
	mux.HandleFunc("GET /health", handlers.Health)

	mux.HandleFunc("GET /api/v1/status", handlers.Status)

	mux.HandleFunc("GET /api/v1/users", dbHandlers.ListUsers)
	mux.HandleFunc("GET /api/v1/users/{id}", dbHandlers.GetUser)
	mux.HandleFunc("GET /api/v1/threats", dbHandlers.ListThreats)
	mux.HandleFunc("GET /api/v1/threats/{id}", dbHandlers.GetThreat)

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	mux.HandleFunc("GET /{$}", handlers.Home)
	mux.HandleFunc("/{path...}", handlers.NotFound)
}
