package tastytrade

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Tastytrade rotates the refresh token on every successful exchange. Without
// persisting the new token back to Secret Manager, the next cold start of the
// container would read the stale value from env and fail with invalid_grant.
// This file implements an OnTokenRotated callback that writes the rotated
// token as a new Secret Manager version using the Cloud Run service account,
// via the GCE metadata server + Secret Manager REST API (no SDK dependency).

const (
	metadataTokenURL   = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
	metadataProjectURL = "http://metadata.google.internal/computeMetadata/v1/project/project-id"
	smAddVersionURLFmt = "https://secretmanager.googleapis.com/v1/projects/%s/secrets/%s:addVersion"
)

func gcpMetadataGet(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	res, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return "", fmt.Errorf("metadata %d: %s", res.StatusCode, truncate(string(body), 200))
	}
	return string(body), nil
}

func gcpMetadataToken(ctx context.Context) (string, error) {
	raw, err := gcpMetadataGet(ctx, metadataTokenURL)
	if err != nil {
		return "", err
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return "", fmt.Errorf("decode metadata token: %w", err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("empty access_token from metadata server")
	}
	return out.AccessToken, nil
}

func addSecretVersion(ctx context.Context, projectID, secretName, payload string) error {
	token, err := gcpMetadataToken(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]interface{}{
		"payload": map[string]string{
			"data": base64.StdEncoding.EncodeToString([]byte(payload)),
		},
	})
	url := fmt.Sprintf(smAddVersionURLFmt, projectID, secretName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("sm addVersion: %w", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return fmt.Errorf("sm addVersion %d: %s", res.StatusCode, truncate(string(b), 300))
	}
	return nil
}

// newGCPSecretPersister returns an OnTokenRotated callback that adds a new
// Secret Manager version for secretName whenever tastytrade rotates the
// refresh token. Returns nil when not running in GCP (metadata server
// unreachable) so local dev silently no-ops.
func newGCPSecretPersister(secretName string, logger *slog.Logger) func(string) {
	if secretName == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	projectID, err := gcpMetadataGet(ctx, metadataProjectURL)
	if err != nil || projectID == "" {
		if logger != nil {
			logger.Info("tastytrade refresh-token persistence disabled (not on GCP)", "error", err)
		}
		return nil
	}
	return func(newToken string) {
		wctx, wcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer wcancel()
		if err := addSecretVersion(wctx, projectID, secretName, newToken); err != nil {
			if logger != nil {
				logger.Warn("tastytrade refresh-token persist failed — next cold start will break",
					"secret", secretName, "error", err)
			}
			return
		}
		if logger != nil {
			logger.Info("tastytrade refresh-token rotation persisted",
				"secret", secretName, "project", projectID)
		}
	}
}
