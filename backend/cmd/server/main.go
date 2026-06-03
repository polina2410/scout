package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/polina2410/scout/backend/internal/config"
	"github.com/polina2410/scout/backend/internal/handler"
	"github.com/polina2410/scout/backend/internal/logger"
	"github.com/polina2410/scout/backend/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}

	log := logger.New(os.Stdout, cfg.LogLevel)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"version": "dev",
		})
	})

	var h http.Handler = mux
	h = middleware.CorrelationID(log)(h)

	log.Info("server starting", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, h); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
