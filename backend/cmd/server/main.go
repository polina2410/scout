package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/polina2410/scout/backend/internal/config"
	"github.com/polina2410/scout/backend/internal/db"
	"github.com/polina2410/scout/backend/internal/handler"
	"github.com/polina2410/scout/backend/internal/logger"
	"github.com/polina2410/scout/backend/internal/middleware"
	minioclient "github.com/polina2410/scout/backend/internal/minio"
)

const version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}

	log := logger.New(os.Stdout, cfg.LogLevel)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	store, err := minioclient.New(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
	)
	if err != nil {
		log.Error("failed to connect to MinIO", "error", err)
		os.Exit(1)
	}

	_ = store // used by handlers in Step 5

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"version": version,
		})
	})

	var h http.Handler = mux
	h = middleware.CorrelationID(log)(h)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // generous for thumbnail streaming (Step 6)
		IdleTimeout:  120 * time.Second,
	}

	log.Info("server starting", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
