package handler

import (
	"log/slog"

	"github.com/polina2410/scout/backend/internal/db"
	minioclient "github.com/polina2410/scout/backend/internal/minio"
)

// App holds the shared dependencies for all API handlers.
type App struct {
	DB    *db.DB
	Store minioclient.Presigner
	Log   *slog.Logger
}
