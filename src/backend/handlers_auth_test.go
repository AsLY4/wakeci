package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func newAuthTestEnv(t *testing.T) {
	t.Helper()
	db, err := bolt.Open(filepath.Join(t.TempDir(), "auth.db"), 0600, nil)
	if err != nil {
		t.Fatalf("open auth test db: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(GlobalBucket)
		if err != nil {
			return err
		}
		return bucket.Put([]byte("password"), hash)
	}); err != nil {
		t.Fatalf("store test password: %v", err)
	}

	originalDB := DB
	originalSessions := GlobalSessionStorage
	originalLimiter := globalAuthAttemptLimiter
	DB = db
	GlobalSessionStorage = &SessionStorage{sessions: make(map[string]time.Time)}
	globalAuthAttemptLimiter = newAuthAttemptLimiter(2, time.Minute, time.Minute)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close auth test db: %v", err)
		}
		DB = originalDB
		GlobalSessionStorage = originalSessions
		globalAuthAttemptLimiter = originalLimiter
	})
}

func TestAuthenticationRateLimit(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		request func() *http.Request
	}{
		{
			name:    "form login",
			handler: http.HandlerFunc(HandleLogIn),
			request: func() *http.Request {
				form := url.Values{"password": {"wrong"}}
				req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req
			},
		},
		{
			name: "basic auth",
			handler: AuthMi(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})),
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/feed", nil)
				req.SetBasicAuth("", "wrong")
				return req
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newAuthTestEnv(t)
			for attempt, wantStatus := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
				req := tt.request()
				req.RemoteAddr = "192.0.2.10:1234"
				response := httptest.NewRecorder()
				tt.handler.ServeHTTP(response, req)
				if response.Code != wantStatus {
					t.Errorf("attempt %d status = %d, want %d", attempt+1, response.Code, wantStatus)
				}
				if wantStatus == http.StatusTooManyRequests {
					if response.Header().Get("Retry-After") == "" {
						t.Error("rate-limited response has no Retry-After header")
					}
					reset, err := strconv.ParseInt(response.Header().Get("X-RateLimit-Reset"), 10, 64)
					if err != nil {
						t.Errorf("X-RateLimit-Reset is not a Unix timestamp: %v", err)
					} else if reset <= time.Now().Unix() {
						t.Errorf("X-RateLimit-Reset = %d, want a future timestamp", reset)
					}
					if response.Header().Get("X-RateLimit-Limit") != "2" {
						t.Errorf("X-RateLimit-Limit = %q, want 2", response.Header().Get("X-RateLimit-Limit"))
					}
				}
			}
		})
	}
}
