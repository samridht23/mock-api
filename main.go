// @title Mock API
// @version 1.0
// @description Mock backend
// @host localhost:8080
// @BasePath /
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/samridht23/mock-api/internal"

	"github.com/samridht23/mock-api/internal/core"
	_ "github.com/samridht23/mock-api/internal/utils" // init validator init()
)

func main() {
	err := godotenv.Load()
	if err != nil {
    slog.Info("no .env file found, using system environment")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	googleProvier := core.NewGoogleProvider()
	auth := core.NewAuthService(pool, googleProvier)
	googleHTTP := core.NewGoogleHTTPService(&http.Client{})

	r := chi.NewRouter()
	internal.InitRoutes(r, pool, auth, googleHTTP)
	initServer(r)
}

func initServer(r *chi.Mux) {
	s := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("failed to listen to port")
		}
	}()
	<-sigCh
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		slog.Error("gracefull shutdown failed", "error", err)
		if err := s.Close(); err != nil {
			slog.Error("forced shutdown failed", "error", err)
		}
		slog.Info("server forced shutdown")
	}
	slog.Info("server shutdown successfully")
}
