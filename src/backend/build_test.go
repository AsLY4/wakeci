package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

// newTestBuildEnv wires up the minimal global state CreateBuild/BroadcastUpdate
// need (Config, DB with the buckets they touch, and a running WSHub).
func newTestBuildEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	Config = &WakeConfig{
		WorkDir: dir + string(os.PathSeparator),
		JobDir:  dir + string(os.PathSeparator),
		jobsExt: ".yaml",
	}

	dbPath := filepath.Join(dir, "test.db")
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(GlobalBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(HistoryBucket); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(JobsBucket)
		return err
	})
	if err != nil {
		t.Fatalf("create buckets: %v", err)
	}

	oldDB := DB
	DB = db
	oldHub := WSHub
	WSHub = newHub()
	go WSHub.run()

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test db: %v", err)
		}
		DB = oldDB
		WSHub = oldHub
	})
}

// TestSetBuildStatusWaitsForPendingTasksBeforeRunning is a regression test:
// pendingTasksWG.Add(1) used to happen inside the goroutine spawned to run
// on_pending tasks, not before it was spawned. A sync.WaitGroup's Add must
// happen-before the matching Wait; since it didn't, SetBuildStatus(Running)'s
// Wait() could return before the on_pending task had even started, letting
// it race the build's main tasks instead of finishing first.
func TestSetBuildStatusWaitsForPendingTasksBeforeRunning(t *testing.T) {
	newTestBuildEnv(t)

	job := &Job{
		Name: "pending-race-test",
		Tasks: []*Task{
			{Kind: string(StatusPending), Name: "slow pending", Command: "sleep 0.3"},
		},
	}

	build, err := CreateBuild(job, filepath.Join(Config.JobDir, "pending-race-test.yaml"))
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}

	start := time.Now()
	build.SetBuildStatus(StatusRunning)
	elapsed := time.Since(start)

	if elapsed < 250*time.Millisecond {
		t.Errorf("SetBuildStatus(StatusRunning) returned after %v; expected it to wait for the ~300ms on_pending task first", elapsed)
	}
}
