package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStorageSecurityMiBlocksNetworkAndFormsFromPreviewedContent is a
// regression test: /storage/build/* serves build artifacts, which can
// contain task-influenced content, with a relaxed CSP so HTML reports can
// still preview. The relaxed policy used to omit connect-src/form-action
// entirely, meaning a crafted artifact could execute a script that called
// back into wakeci's own authenticated API (change password, run/delete
// jobs) using the session cookie, which is still attached to a same-site
// request even under SameSite=Strict.
func TestStorageSecurityMiBlocksNetworkAndFormsFromPreviewedContent(t *testing.T) {
	Config = &WakeConfig{}
	handler := StorageSecurityMi(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/storage/build/1/artifacts/report.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("content-security-policy")
	if csp == "" {
		t.Fatal("expected a content-security-policy header to be set")
	}
	for _, want := range []string{"connect-src 'none'", "form-action 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("expected CSP to contain %q, got %q", want, csp)
		}
	}
}

// TestStorageSecurityMiStillAllowsInlineScriptForReportPreviews verifies the
// fix doesn't regress the feature it's explicitly meant to preserve:
// self-contained HTML reports with inline script/style must still render.
func TestStorageSecurityMiStillAllowsInlineScriptForReportPreviews(t *testing.T) {
	Config = &WakeConfig{}
	handler := StorageSecurityMi(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/storage/build/1/artifacts/report.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("content-security-policy")
	for _, want := range []string{"script-src 'self' 'unsafe-inline'", "style-src 'self' 'unsafe-inline'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("expected CSP to still allow inline script/style for report previews, got %q", csp)
		}
	}
}

func TestStorageSecurityMiPinsLogContentType(t *testing.T) {
	Config = &WakeConfig{}
	handler := StorageSecurityMi(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/storage/build/1/task_0.log", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if ct := rec.Header().Get("content-type"); ct != "text/plain" {
		t.Errorf("expected content-type text/plain for a .log file, got %q", ct)
	}
}
