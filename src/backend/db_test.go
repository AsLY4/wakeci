package main

import (
	"bytes"
	"testing"
)

func TestByteToInt(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		expected  int
		expectErr bool
	}{
		{name: "Zero", input: []byte("0"), expected: 0},
		{name: "Positive", input: []byte("42"), expected: 42},
		{name: "NotANumber", input: []byte("abc"), expectErr: true},
		{name: "Empty", input: []byte(""), expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ByteToInt(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Error("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestIntToByte(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{name: "Zero", input: 0, expected: "0"},
		{name: "Positive", input: 42, expected: "42"},
		{name: "Negative", input: -7, expected: "-7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(IntToByte(tt.input))
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestItob(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected []byte
	}{
		{name: "Zero", input: 0, expected: []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "One", input: 1, expected: []byte{0, 0, 0, 0, 0, 0, 0, 1}},
		{name: "256", input: 256, expected: []byte{0, 0, 0, 0, 0, 0, 1, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Itob(tt.input)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestItobOrdering(t *testing.T) {
	// Itob must produce a byte-sorted representation, since it is used as a
	// bbolt key so that iteration order matches numeric order.
	small := Itob(1)
	large := Itob(2)
	if bytes.Compare(small, large) >= 0 {
		t.Errorf("expected Itob(1) to sort before Itob(2), got %v >= %v", small, large)
	}
}
