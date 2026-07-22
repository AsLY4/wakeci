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
	originalQueue := GlobalQueue
	Config = &WakeConfig{WorkDir: dir + string(os.PathSeparator)}
	GlobalQueue = &Queue{}
	t.Cleanup(func() {
		Config = originalConfig
		DB = originalDB
		GlobalQueue = originalQueue
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

func TestCleanerPreservesActiveExpiredBuild(t *testing.T) {
	dir := t.TempDir()
	originalConfig := Config
	originalDB := DB
	originalQueue := GlobalQueue
	Config = &WakeConfig{WorkDir: dir + string(os.PathSeparator)}
	GlobalQueue = &Queue{running: []*Build{{ID: 1}}}
	t.Cleanup(func() {
		Config = originalConfig
		DB = originalDB
		GlobalQueue = originalQueue
	})

	db, err := bolt.Open(filepath.Join(dir, "active-cleanup.db"), 0600, nil)
	if err != nil {
		t.Fatalf("open active cleanup test db: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close active cleanup test db: %v", err)
		}
	})

	if err := db.Update(func(tx *bolt.Tx) error {
		global, err := tx.CreateBucketIfNotExists(GlobalBucket)
		if err != nil {
			return err
		}
		if err := global.Put([]byte("buildHistorySize"), IntToByte(1)); err != nil {
			return err
		}
		history, err := tx.CreateBucketIfNotExists(HistoryBucket)
		if err != nil {
			return err
		}
		for id := 1; id <= 3; id++ {
			if err := history.Put(Itob(id), []byte("build")); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed active cleanup history: %v", err)
	}

	for _, id := range []string{"1", "2", "3"} {
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
		for _, id := range []int{1, 3} {
			if history.Get(Itob(id)) == nil {
				t.Errorf("build %d should have been preserved", id)
			}
		}
		if history.Get(Itob(2)) != nil {
			t.Error("inactive expired build 2 was not deleted")
		}
		return nil
	}); err != nil {
		t.Fatalf("read active cleanup history: %v", err)
	}

	for _, root := range []string{"workspace", "wakespace"} {
		if _, err := os.Stat(filepath.Join(dir, root, "1")); err != nil {
			t.Errorf("active build path was removed: %s: %v", root, err)
		}
		if _, err := os.Stat(filepath.Join(dir, root, "2")); !os.IsNotExist(err) {
			t.Errorf("inactive expired build path still exists: %s", root)
		}
	}
}
