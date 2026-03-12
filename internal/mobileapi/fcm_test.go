package mobileapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

type fakeTokenSource struct{}

func (fakeTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "token"}, nil
}

func TestFCMSenderSendToKey(t *testing.T) {
	store := NewPushTokenStore(t.TempDir() + "/push_tokens.json")
	if err := store.Put("supplier:SUP-001", "device-token", "android"); err != nil {
		t.Fatalf("seed push token: %v", err)
	}

	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen = true
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := &fcmSender{
		store:      store,
		httpClient: server.Client(),
		tokenSrc:   fakeTokenSource{},
		projectID:  "demo",
		endpoint:   server.URL,
	}

	if err := sender.SendToKey(context.Background(), "supplier:SUP-001", "Title", "Body", map[string]string{"id": "1"}); err != nil {
		t.Fatalf("SendToKey() error = %v", err)
	}
	if !requestSeen {
		t.Fatal("expected request to be sent")
	}
}

func TestDiscoverServiceAccountPathReturnsFirebaseAdminSDKFile(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/demo-firebase-adminsdk.json"
	if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(prev)

	got := discoverServiceAccountPath()
	if !strings.Contains(got, "firebase-adminsdk") {
		t.Fatalf("unexpected path: %s", got)
	}
}

func TestFCMSenderSkipsStaleTokenAndDeliversToNext(t *testing.T) {
	store := NewPushTokenStore(t.TempDir() + "/push_tokens.json")
	if err := store.Put("supplier:SUP-001", "stale-token", "android"); err != nil {
		t.Fatalf("seed stale token: %v", err)
	}
	if err := store.Put("supplier:SUP-001", "fresh-token", "android"); err != nil {
		t.Fatalf("seed fresh token: %v", err)
	}

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		rawBytes, _ := io.ReadAll(r.Body)
		raw := string(rawBytes)
		switch {
		case strings.Contains(raw, "stale-token"):
			http.Error(w, `{"error":{"message":"Requested entity was not found.","status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`, http.StatusNotFound)
		case strings.Contains(raw, "fresh-token"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.Error(w, fmt.Sprintf("unexpected body: %s", raw), http.StatusBadRequest)
		}
	}))
	defer server.Close()

	sender := &fcmSender{
		store:      store,
		httpClient: server.Client(),
		tokenSrc:   fakeTokenSource{},
		projectID:  "demo",
		endpoint:   server.URL,
	}

	if err := sender.SendToKey(context.Background(), "supplier:SUP-001", "Title", "Body", map[string]string{"id": "1"}); err != nil {
		t.Fatalf("SendToKey() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 send attempts, got %d", requests)
	}
	remaining, err := store.List("supplier:SUP-001")
	if err != nil {
		t.Fatalf("store.List() error = %v", err)
	}
	if len(remaining) != 1 || remaining[0].Token != "fresh-token" {
		t.Fatalf("expected stale token to be dropped, got %+v", remaining)
	}
}
