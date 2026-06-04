package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	minioclient "github.com/polina2410/scout/backend/internal/minio"
)

const httpTimeout = 5 * time.Minute

type seedConfig struct {
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
	APIKey         string
	APIURL         string
	ImagesDir      string
}

type uploadLinkResponse struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// existenceChecker is satisfied by *minioclient.Client.
type existenceChecker interface {
	ObjectExists(ctx context.Context, photoID string) (bool, error)
}

func loadConfig() (*seedConfig, error) {
	var missing []string
	get := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			missing = append(missing, name)
		}
		return v
	}

	cfg := &seedConfig{
		MinIOEndpoint:  get("MINIO_ENDPOINT"),
		MinIOAccessKey: get("MINIO_ACCESS_KEY"),
		MinIOSecretKey: get("MINIO_SECRET_KEY"),
		MinIOBucket:    get("MINIO_BUCKET"),
		APIKey:         get("API_KEY"),
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("required env vars not set: %s", strings.Join(missing, ", "))
	}

	cfg.MinIOUseSSL = strings.EqualFold(os.Getenv("MINIO_USE_SSL"), "true")

	cfg.APIURL = os.Getenv("API_URL")
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:8080"
	}

	cfg.ImagesDir = os.Getenv("IMAGES_DIR")
	if cfg.ImagesDir == "" {
		cfg.ImagesDir = "../dataset/images"
	}

	return cfg, nil
}

// uploadPhoto checks existence, fetches a presigned PUT URL, and uploads one photo.
// Returns (true, nil) if skipped (already exists), (false, nil) on success, (false, err) on failure.
func uploadPhoto(
	ctx context.Context,
	store existenceChecker,
	httpCl *http.Client,
	apiURL string,
	apiKey string,
	photoID string,
	imagePath string,
) (skipped bool, err error) {
	exists, err := store.ObjectExists(ctx, photoID)
	if err != nil {
		return false, fmt.Errorf("check existence: %w", err)
	}
	if exists {
		return true, nil
	}

	body, err := json.Marshal(map[string]string{"contentType": "image/jpeg"})
	if err != nil {
		return false, fmt.Errorf("marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		apiURL+"/photos/"+photoID+"/upload-link",
		bytes.NewReader(body),
	)
	if err != nil {
		return false, fmt.Errorf("build upload-link request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := httpCl.Do(req)
	if err != nil {
		return false, fmt.Errorf("POST upload-link: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("POST upload-link returned %d", resp.StatusCode)
	}

	var link uploadLinkResponse
	if err := json.NewDecoder(resp.Body).Decode(&link); err != nil {
		return false, fmt.Errorf("decode upload-link response: %w", err)
	}

	data, err := os.ReadFile(imagePath)
	if err != nil {
		return false, fmt.Errorf("read image: %w", err)
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, link.URL, bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("build PUT request: %w", err)
	}
	for k, v := range link.Headers {
		putReq.Header.Set(k, v)
	}

	putResp, err := httpCl.Do(putReq)
	if err != nil {
		return false, fmt.Errorf("PUT to object storage: %w", err)
	}
	io.Copy(io.Discard, putResp.Body) //nolint:errcheck
	putResp.Body.Close()
	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		return false, fmt.Errorf("PUT returned %d", putResp.StatusCode)
	}

	return false, nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	store, err := minioclient.New(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MinIO: %v\n", err)
		os.Exit(1)
	}

	images, err := filepath.Glob(filepath.Join(cfg.ImagesDir, "*.jpg"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "glob images: %v\n", err)
		os.Exit(1)
	}
	if len(images) == 0 {
		fmt.Fprintf(os.Stderr, "no .jpg files found in %s\n", cfg.ImagesDir)
		os.Exit(1)
	}

	httpCl := &http.Client{Timeout: httpTimeout}
	ctx := context.Background()

	var uploaded, skipped, errors int
	for _, path := range images {
		base := filepath.Base(path)
		photoID := strings.TrimSuffix(base, ".jpg")
		if _, err := uuid.Parse(photoID); err != nil {
			log.Printf("skip %s: filename is not a valid UUID", base)
			skipped++
			continue
		}

		skip, err := uploadPhoto(ctx, store, httpCl, cfg.APIURL, cfg.APIKey, photoID, path)
		if err != nil {
			log.Printf("error %s: %v", photoID, err)
			errors++
			continue
		}
		if skip {
			log.Printf("skip %s", photoID)
			skipped++
		} else {
			log.Printf("uploaded %s", photoID)
			uploaded++
		}
	}

	fmt.Printf("done: %d uploaded, %d skipped, %d errors\n", uploaded, skipped, errors)
	if errors > 0 {
		os.Exit(1)
	}
}
