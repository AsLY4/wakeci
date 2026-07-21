package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetJobNameFromPath(t *testing.T) {
	Config = &WakeConfig{jobsExt: ".yaml"}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{name: "SimpleName", path: "/jobs/build.yaml", expected: "build"},
		{name: "RelativePath", path: "build.yaml", expected: "build"},
		{name: "NameWithDashes", path: "/jobs/nightly-build.yaml", expected: "nightly-build"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetJobNameFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCreateJobFromFile(t *testing.T) {
	dir := t.TempDir()
	Config = &WakeConfig{JobDir: dir, jobsExt: ".yaml"}

	tests := []struct {
		name          string
		content       string
		expectErr     bool
		expectedTasks int
	}{
		{
			name: "SimpleJob",
			content: `desc: A test job
tasks:
  - name: step one
    run: echo one
  - name: step two
    run: echo two
`,
			expectedTasks: 2,
		},
		{
			name: "WithOnFailedAndFinally",
			content: `desc: A job with hooks
tasks:
  - name: main step
    run: echo main
on_failed:
  - name: notify
    run: echo failed
finally:
  - name: cleanup
    run: echo cleanup
`,
			expectedTasks: 3,
		},
		{
			name:      "InvalidYaml",
			content:   "tasks: [this is not valid",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := tt.name + ".yaml"
			path := filepath.Join(dir, filename)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			job, err := CreateJobFromFile(path)
			if tt.expectErr {
				if err == nil {
					t.Error("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if job.Name != tt.name {
				t.Errorf("expected job name %q, got %q", tt.name, job.Name)
			}
			if len(job.Tasks) != tt.expectedTasks {
				t.Errorf("expected %d tasks, got %d", tt.expectedTasks, len(job.Tasks))
			}
			for i, task := range job.Tasks {
				if task.ID != i {
					t.Errorf("expected task %d to have ID %d, got %d", i, i, task.ID)
				}
				if task.Status != StatusPending {
					t.Errorf("expected task %d to start pending, got %s", i, task.Status)
				}
			}
		})
	}
}
