package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateWakeConfig(t *testing.T) {
	t.Run("Defaults", func(t *testing.T) {
		config, err := CreateWakeConfig(filepath.Join(t.TempDir(), "missing.yaml"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Port != "8081" {
			t.Errorf("expected default port 8081, got %q", config.Port)
		}
		if !filepath.IsAbs(config.WorkDir) {
			t.Errorf("expected WorkDir to be made absolute, got %q", config.WorkDir)
		}
		if !filepath.IsAbs(config.JobDir) {
			t.Errorf("expected JobDir to be made absolute, got %q", config.JobDir)
		}
	})

	t.Run("FromFile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "Wakefile.yaml")
		content := "port: \"9090\"\nhostname: example.com\ntimezone: UTC\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		config, err := CreateWakeConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Port != "9090" {
			t.Errorf("expected port 9090, got %q", config.Port)
		}
		if config.Hostname != "example.com" {
			t.Errorf("expected hostname example.com, got %q", config.Hostname)
		}
		if config.Timezone != "UTC" {
			t.Errorf("expected timezone UTC, got %q", config.Timezone)
		}
	})

	t.Run("InvalidYaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "Wakefile.yaml")
		if err := os.WriteFile(path, []byte("port: [this is not valid"), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		if _, err := CreateWakeConfig(path); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("AbsoluteDirsWithoutTrailingSlash", func(t *testing.T) {
		// WorkDir/JobDir are joined with filenames via plain string
		// concatenation elsewhere (e.g. Config.JobDir+name+Config.jobsExt),
		// so an absolute path configured without a trailing separator must
		// still end up with one - regression test for a bug where only
		// relative paths got normalized this way.
		dir := t.TempDir()
		workDir := filepath.Join(dir, "workdir")
		jobDir := filepath.Join(dir, "jobdir")
		path := filepath.Join(dir, "Wakefile.yaml")
		content := "workdir: " + workDir + "\njobdir: " + jobDir + "\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		config, err := CreateWakeConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(config.WorkDir, string(os.PathSeparator)) {
			t.Errorf("expected WorkDir to end with a separator, got %q", config.WorkDir)
		}
		if !strings.HasSuffix(config.JobDir, string(os.PathSeparator)) {
			t.Errorf("expected JobDir to end with a separator, got %q", config.JobDir)
		}
	})
}
