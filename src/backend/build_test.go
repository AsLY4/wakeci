package main

import (
	"bytes"
	"os"
	"os/exec"
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

func TestCollectArtifactsRedactsSecrets(t *testing.T) {
	originalConfig := Config
	t.Cleanup(func() { Config = originalConfig })

	dir := t.TempDir()
	Config = &WakeConfig{
		WorkDir: dir + string(os.PathSeparator),
		secrets: map[string]string{"TOKEN": "top-secret-value"},
	}
	build := &Build{
		ID:     7,
		Job:    &Job{Artifacts: []string{"result.bin"}},
		Logger: L,
	}
	if err := os.MkdirAll(build.GetWorkspaceDir(), 0755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.MkdirAll(build.GetArtifactsDir(), 0755); err != nil {
		t.Fatalf("create artifacts directory: %v", err)
	}

	prefix := bytes.Repeat([]byte("x"), 32*1024-5)
	artifact := append(prefix, []byte("top-secret-value\x00tail")...)
	source := filepath.Join(build.GetWorkspaceDir(), "result.bin")
	if err := os.WriteFile(source, artifact, 0644); err != nil {
		t.Fatalf("write source artifact: %v", err)
	}

	build.CollectArtifacts()

	result, err := os.ReadFile(filepath.Join(build.GetArtifactsDir(), "result.bin"))
	if err != nil {
		t.Fatalf("read collected artifact: %v", err)
	}
	expected := append(prefix, []byte(redactedSecret+"\x00tail")...)
	if !bytes.Equal(result, expected) {
		t.Errorf("collected artifact was not redacted across the copy buffer boundary")
	}
	if len(build.BuildArtifacts) != 1 {
		t.Fatalf("BuildArtifacts length = %d, want 1", len(build.BuildArtifacts))
	}
	if build.BuildArtifacts[0].Size != int64(len(expected)) {
		t.Errorf("artifact size = %d, want %d", build.BuildArtifacts[0].Size, len(expected))
	}
}

func TestWaitForCondition(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		timeout      time.Duration
		wantTimedOut bool
		wantError    bool
	}{
		{name: "command completes", command: "true", timeout: time.Second},
		{name: "command is false", command: "false", timeout: time.Second, wantError: true},
		{name: "command times out", command: "sleep 1", timeout: 20 * time.Millisecond, wantTimedOut: true, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := exec.Command("bash", "-c", tt.command)
			if err := command.Start(); err != nil {
				t.Fatalf("start condition: %v", err)
			}
			build := &Build{Logger: L}
			err, timedOut := build.waitForCondition(command, 1, tt.timeout)
			if timedOut != tt.wantTimedOut {
				t.Errorf("timedOut = %t, want %t", timedOut, tt.wantTimedOut)
			}
			if (err != nil) != tt.wantError {
				t.Errorf("error = %v, wantError %t", err, tt.wantError)
			}
		})
	}
}

func TestRunTaskInjectsSecretsIntoConditions(t *testing.T) {
	newTestBuildEnv(t)
	Config.secrets = map[string]string{"FLAG": "enabled"}

	job := &Job{Name: "condition-secrets-test"}
	build, err := CreateBuild(job, filepath.Join(Config.JobDir, "condition-secrets-test.yaml"))
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}

	tests := []struct {
		name string
		task *Task
	}{
		{
			name: "when",
			task: &Task{ID: 1, When: "{{ secrets.FLAG }} == enabled", Command: "true"},
		},
		{
			name: "if",
			task: &Task{ID: 2, If: `test "{{ secrets.FLAG }}" = enabled`, Command: "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.startedAt = time.Now()
			if status := build.runTask(tt.task); status != StatusFinished {
				t.Errorf("runTask status = %q, want %q", status, StatusFinished)
			}
		})
	}
}
