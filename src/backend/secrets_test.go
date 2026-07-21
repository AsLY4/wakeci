package main

import (
	"strings"
	"testing"
)

func withSecrets(secrets map[string]string, fn func()) {
	old := Config
	Config = &WakeConfig{secrets: secrets}
	defer func() { Config = old }()
	fn()
}

func TestInjectSecrets(t *testing.T) {
	tests := []struct {
		name     string
		secrets  map[string]string
		input    string
		expected string
	}{
		{
			name:     "KnownKey",
			secrets:  map[string]string{"API_KEY": "s3cr3t"},
			input:    "token={{ secrets.API_KEY }}",
			expected: "token=s3cr3t",
		},
		{
			name:     "NoWhitespaceBeforeKey",
			secrets:  map[string]string{"API_KEY": "s3cr3t"},
			input:    "token={{secrets.API_KEY }}",
			expected: "token=s3cr3t",
		},
		{
			name:     "ExtraWhitespace",
			secrets:  map[string]string{"API_KEY": "s3cr3t"},
			input:    "token={{     secrets.API_KEY }}",
			expected: "token=s3cr3t",
		},
		{
			name:     "UnknownKey",
			secrets:  map[string]string{"API_KEY": "s3cr3t"},
			input:    "token={{secrets.UNKNOWN_KEY }}",
			expected: "token=<no value>",
		},
		{
			name:     "MultipleReferences",
			secrets:  map[string]string{"A": "one", "B": "two"},
			input:    "{{ secrets.A }} and {{ secrets.B }}",
			expected: "one and two",
		},
		{
			name:     "NoReferences",
			secrets:  map[string]string{"A": "one"},
			input:    "nothing to see here",
			expected: "nothing to see here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withSecrets(tt.secrets, func() {
				result := injectSecrets(tt.input)
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			})
		})
	}
}

// TestInjectSecretsCannotReferenceAnUnnamedSecret is a regression test:
// injectSecrets used to run the whole input string through text/template
// against the entire Config.secrets map, so a single {{ secrets.X }}
// reference anywhere in the string unlocked arbitrary template actions
// elsewhere in that same string - including referencing a secret other than
// the one named. This must now be inert, literal text.
func TestInjectSecretsCannotReferenceAnUnnamedSecret(t *testing.T) {
	withSecrets(map[string]string{"KNOWN": "safe-value", "OTHER": "top-secret"}, func() {
		// KNOWN is a valid reference (required to reach the vulnerable code
		// path in the old implementation); the .OTHER field access piggybacks
		// on the same template execution in the old, vulnerable version.
		input := "{{ secrets.KNOWN }}{{ .OTHER }}"
		result := injectSecrets(input)
		if strings.Contains(result, "top-secret") {
			t.Fatalf("expected OTHER to never leak, got %q", result)
		}
		if !strings.Contains(result, "safe-value") {
			t.Errorf("expected the named secret to still be substituted, got %q", result)
		}
	})
}

// TestInjectSecretsCannotDumpAllSecrets is a regression test for the same
// root cause: a {{ range $k, $v := . }} action riding along with one valid
// {{ secrets.X }} reference used to dump every configured secret.
func TestInjectSecretsCannotDumpAllSecrets(t *testing.T) {
	withSecrets(map[string]string{"A": "secret-a", "B": "secret-b"}, func() {
		input := "{{ secrets.A }}{{ range $k, $v := . }}{{$k}}={{$v}};{{end}}"
		result := injectSecrets(input)
		if strings.Contains(result, "secret-b") {
			t.Fatalf("expected B to never leak via a range action, got %q", result)
		}
		if !strings.Contains(result, "secret-a") {
			t.Errorf("expected the named secret to still be substituted, got %q", result)
		}
		// The range/end action itself must be left untouched as literal text,
		// not executed.
		if !strings.Contains(result, "{{ range $k, $v := . }}") {
			t.Errorf("expected the range action to be left as inert literal text, got %q", result)
		}
	})
}

func TestRedactSecrets(t *testing.T) {
	withSecrets(map[string]string{"API_KEY": "s3cr3t"}, func() {
		result := redactSecrets("the key is s3cr3t, really")
		expected := "the key is ***REDACTED***, really"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})
}
