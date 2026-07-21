package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTasks(t *testing.T) {
	dir := t.TempDir()
	Config = &WakeConfig{JobDir: dir}

	content := []byte("- name: included task\n  run: echo hi\n")
	if err := os.WriteFile(filepath.Join(dir, "included.yaml"), content, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		expectLen int
		expectErr bool
	}{
		{name: "RelativePath", path: "included.yaml", expectLen: 1},
		{name: "AbsolutePath", path: filepath.Join(dir, "included.yaml"), expectLen: 1},
		{name: "MissingFile", path: "missing.yaml", expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := ReadTasks(tt.path)
			if tt.expectErr {
				if err == nil {
					t.Error("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(tasks) != tt.expectLen {
				t.Errorf("expected %d tasks, got %d", tt.expectLen, len(tasks))
			}
		})
	}
}

func TestExpandTasks(t *testing.T) {
	dir := t.TempDir()
	Config = &WakeConfig{JobDir: dir}

	includedContent := []byte("- name: from include\n  run: echo included\n")
	if err := os.WriteFile(filepath.Join(dir, "included.yaml"), includedContent, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	t.Run("NoExpansion", func(t *testing.T) {
		tasks := []*Task{{Name: "plain", Command: "echo hi"}}
		if err := ExpandTasks(&tasks); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tasks) != 1 || tasks[0].Name != "plain" {
			t.Errorf("expected task list to stay unchanged, got %+v", tasks)
		}
	})

	t.Run("Include", func(t *testing.T) {
		tasks := []*Task{{IncludePath: "included.yaml"}}
		if err := ExpandTasks(&tasks); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tasks) != 1 || tasks[0].Name != "from include" {
			t.Errorf("expected the include to be replaced by its task, got %+v", tasks)
		}
	})

	t.Run("IncludeMissingFile", func(t *testing.T) {
		tasks := []*Task{{IncludePath: "missing.yaml"}}
		if err := ExpandTasks(&tasks); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("Block", func(t *testing.T) {
		tasks := []*Task{{
			Name: "wrapper",
			When: "1 == 1",
			Block: []*Task{
				{Name: "inner", Command: "echo inner"},
			},
		}}
		if err := ExpandTasks(&tasks); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tasks) != 1 || tasks[0].Name != "inner" {
			t.Fatalf("expected the block to be replaced by its inner task, got %+v", tasks)
		}
		if tasks[0].When != "1 == 1" {
			t.Errorf("expected the wrapper's `when` condition to carry over, got %q", tasks[0].When)
		}
	})
}
