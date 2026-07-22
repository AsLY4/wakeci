package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	bolt "go.etcd.io/bbolt"
)

func TestHandleGetBuildReturnsNotFoundForMissingHistory(t *testing.T) {
	dir := t.TempDir()
	originalConfig := Config
	originalDB := DB
	Config = &WakeConfig{
		WorkDir: dir + string(os.PathSeparator),
		jobsExt: ".yaml",
	}
	t.Cleanup(func() {
		Config = originalConfig
		DB = originalDB
	})

	buildDir := filepath.Join(dir, "wakespace", "42")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("create build directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "build_plan.yaml"), []byte("name: test\ntasks: []\n"), 0644); err != nil {
		t.Fatalf("write build plan: %v", err)
	}

	db, err := bolt.Open(filepath.Join(dir, "build.db"), 0600, nil)
	if err != nil {
		t.Fatalf("open build test db: %v", err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(HistoryBucket)
		return err
	}); err != nil {
		t.Fatalf("create history bucket: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close build test db: %v", err)
		}
	})

	router := chi.NewRouter()
	router.Get("/api/build/{id}", HandleGetBuild)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/build/42", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
