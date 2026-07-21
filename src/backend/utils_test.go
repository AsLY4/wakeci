package main

import (
	"strings"
	"testing"
)

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{name: "Zero", n: 0},
		{name: "Short", n: 5},
		{name: "Long", n: 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRandomString(tt.n)
			if len(result) != tt.n {
				t.Errorf("expected length %d, got %d (%q)", tt.n, len(result), result)
			}
			for _, c := range result {
				if !strings.ContainsRune(letterBytes, c) {
					t.Errorf("unexpected character %q in result %q", c, result)
				}
			}
		})
	}
}

func TestStripColor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "NoColor", input: "hello world", expected: "hello world"},
		{name: "Empty", input: "", expected: ""},
		{name: "RedText", input: "\x1b[31mhello\x1b[0m", expected: "hello"},
		{name: "MultipleCodes", input: "\x1b[1m\x1b[32mgreen bold\x1b[0m", expected: "green bold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripColor(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNormalizeNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "Unix", input: "one\ntwo\n", expected: "one\ntwo\n"},
		{name: "Windows", input: "one\r\ntwo\r\n", expected: "one\ntwo\n"},
		{name: "Mac", input: "one\rtwo\r", expected: "one\ntwo\n"},
		{name: "Mixed", input: "one\r\ntwo\rthree\n", expected: "one\ntwo\nthree\n"},
		{name: "NoNewlines", input: "one line", expected: "one line"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeNewlines([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}
