package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the server.
// All fields are populated by Load(); callers must treat it as read-only.
type Config struct {
	Port             string
	APIKey           string
	DBPath           string
	MinIOEndpoint    string
	MinIOAccessKey   string
	MinIOSecretKey   string
	MinIOBucket      string
	MinIOUseSSL      bool
	ThumbCacheSizeMB int64
	ThumbRatePerSec  float64
	ThumbRateBurst   float64
	LogLevel         string
}

// Load reads environment variables and returns a populated Config.
// It returns an error listing every missing required variable — not just the first —
// so operators see all gaps in a single restart cycle.
func Load() (*Config, error) {
	var missing []string

	get := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			missing = append(missing, name)
		}
		return v
	}

	cfg := &Config{}
	cfg.APIKey = get("API_KEY")
	cfg.DBPath = get("DB_PATH")
	cfg.MinIOEndpoint = get("MINIO_ENDPOINT")
	cfg.MinIOAccessKey = get("MINIO_ACCESS_KEY")
	cfg.MinIOSecretKey = get("MINIO_SECRET_KEY")
	cfg.MinIOBucket = get("MINIO_BUCKET")

	if len(missing) > 0 {
		return nil, fmt.Errorf("required env vars not set: %s", strings.Join(missing, ", "))
	}

	cfg.Port = os.Getenv("PORT")
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	cfg.MinIOUseSSL = strings.EqualFold(os.Getenv("MINIO_USE_SSL"), "true")

	thumbStr := os.Getenv("THUMB_CACHE_SIZE_MB")
	if thumbStr == "" {
		cfg.ThumbCacheSizeMB = 500
	} else {
		v, err := strconv.ParseInt(thumbStr, 10, 64)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("THUMB_CACHE_SIZE_MB must be a positive integer, got %q", thumbStr)
		}
		cfg.ThumbCacheSizeMB = v
	}

	// Per-IP rate limit for the unauthenticated /thumbnails endpoint. The burst
	// is generous enough for a full gallery grid to load at once; the refill rate
	// throttles sustained floods that would otherwise saturate the 4-slot
	// generation semaphore.
	ratePerSec, err := positiveFloatEnv("THUMB_RATE_PER_SEC", 30)
	if err != nil {
		return nil, err
	}
	cfg.ThumbRatePerSec = ratePerSec

	burst, err := positiveFloatEnv("THUMB_RATE_BURST", 60)
	if err != nil {
		return nil, err
	}
	cfg.ThumbRateBurst = burst

	cfg.LogLevel = strings.ToLower(os.Getenv("LOG_LEVEL"))
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	return cfg, nil
}

// positiveFloatEnv reads name as a positive float, returning def when unset.
func positiveFloatEnv(name string, def float64) (float64, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("%s must be a positive number, got %q", name, raw)
	}
	return v, nil
}
