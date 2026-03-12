package mobileapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type pushSender interface {
	SendToKey(ctx context.Context, key, title, body string, data map[string]string) error
}

type noopPushSender struct{}

func (noopPushSender) SendToKey(context.Context, string, string, string, map[string]string) error {
	return nil
}

type fcmSender struct {
	store      *PushTokenStore
	httpClient *http.Client
	tokenSrc   interface {
		Token() (*oauth2.Token, error)
	}
	projectID string
	endpoint  string
}

func newPushSender(store *PushTokenStore) pushSender {
	path := discoverServiceAccountPath()
	if path == "" {
		log.Printf("push sender disabled: no firebase admin sdk json found")
		return noopPushSender{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("push sender disabled: read service account failed: %v", err)
		return noopPushSender{}
	}
	creds, err := google.CredentialsFromJSON(context.Background(), raw, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		log.Printf("push sender disabled: parse service account failed: %v", err)
		return noopPushSender{}
	}
	var meta struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil || strings.TrimSpace(meta.ProjectID) == "" {
		log.Printf("push sender disabled: project_id missing in service account")
		return noopPushSender{}
	}
	log.Printf("push sender enabled for project %s", strings.TrimSpace(meta.ProjectID))
	return &fcmSender{
		store:      store,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		tokenSrc:   creds.TokenSource,
		projectID:  strings.TrimSpace(meta.ProjectID),
		endpoint:   fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", strings.TrimSpace(meta.ProjectID)),
	}
}

func discoverServiceAccountPath() string {
	if env := strings.TrimSpace(os.Getenv("FCM_SERVICE_ACCOUNT_PATH")); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	candidates := []string{"service-account.json"}
	matches, _ := filepath.Glob("*firebase-adminsdk*.json")
	candidates = append(matches, candidates...)
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func (s *fcmSender) SendToKey(ctx context.Context, key, title, body string, data map[string]string) error {
	if s == nil {
		return nil
	}
	tokens, err := s.store.List(strings.TrimSpace(key))
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		log.Printf("push sender skipped: no tokens for %s", strings.TrimSpace(key))
		return nil
	}
	token, err := s.tokenSrc.Token()
	if err != nil {
		return err
	}
	log.Printf("push sender sending to %s (%d token(s))", strings.TrimSpace(key), len(tokens))
	for _, item := range tokens {
		payload := map[string]interface{}{
			"message": map[string]interface{}{
				"token": item.Token,
				"notification": map[string]string{
					"title": title,
					"body":  body,
				},
				"data": data,
				"android": map[string]interface{}{
					"priority": "HIGH",
					"notification": map[string]string{
						"channel_id": "accord_updates",
						"sound":      "default",
					},
				},
			},
		}
		raw, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return fmt.Errorf("fcm send failed with status %d", resp.StatusCode)
		}
		_ = resp.Body.Close()
	}
	return nil
}
