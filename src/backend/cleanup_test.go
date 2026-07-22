package main

import (
	"os"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestCleanerRemovesEveryExpiredBuild(t *testing.T) {
	dir := t.TempDir()
	originalConfig := Config
	originalDB := DB
	Config = &WakeConfig{WorkDir: dir + string(os.PathSeparator)}
	t.Cleanup(func() {
		Config = originalConfig
		DB = originalDB
	})

	db, err := bolt.Open(filepath.Join(dir, "cleanup.db"), 0600, nil)
	if err != nil {
		t.Fatalf("open cleanup test db: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close cleanup test db: %v", err)
		}
	})

	const (
		buildCount = 1000
		preserve   = 10
	)
	if err := db.Update(func(tx *bolt.Tx) error {
		global, err := tx.CreateBucketIfNotExists(GlobalBucket)
		if err != nil {
			return err
		}
		if err := global.Put([]byte("buildHistorySize"), IntToByte(preserve)); err != nil {
			return err
		}
		history, err := tx.CreateBucketIfNotExists(HistoryBucket)
		if err != nil {
			return err
		}
		value := make([]byte, 1024)
		for id := 1; id <= buildCount; id++ {
			if err := history.Put(Itob(id), value); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed build history: %v", err)
	}

	for _, id := range []string{"1", "990", "1000"} {
		if err := os.MkdirAll(filepath.Join(dir, "workspace", id), 0755); err != nil {
			t.Fatalf("create workspace %s: %v", id, err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "wakespace", id), 0755); err != nil {
			t.Fatalf("create wakespace %s: %v", id, err)
		}
	}

	cleaner := Cleaner{Logger: L}
	cleaner.Clean()

	if err := db.View(func(tx *bolt.Tx) error {
		history := tx.Bucket(HistoryBucket)
		if got := history.Stats().KeyN; got != preserve {
			t.Errorf("history entries = %d, want %d", got, preserve)
		}
		for id := buildCount - preserve + 1; id <= buildCount; id++ {
			if history.Get(Itob(id)) == nil {
				t.Errorf("preserved build %d is missing", id)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("read cleaned history: %v", err)
	}

	for _, path := range []string{
		filepath.Join(dir, "workspace", "1"),
		filepath.Join(dir, "workspace", "990"),
		filepath.Join(dir, "wakespace", "1"),
		filepath.Join(dir, "wakespace", "990"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expired build path still exists: %s", path)
		}
	}
	for _, path := range []string{
		filepath.Join(dir, "workspace", "1000"),
		filepath.Join(dir, "wakespace", "1000"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("preserved build path is unavailable: %s: %v", path, err)
		}
	}
}
