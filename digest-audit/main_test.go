package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSampleFile(t *testing.T) {
	// Get absolute path to sample directory
	samplePath, err := filepath.Abs("sample")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Run audit on sample directory
	uses, err := auditDigestUses(samplePath)
	if err != nil {
		t.Fatalf("Failed to audit sample: %v", err)
	}

	// Read expected output from fixture
	expectedPath := filepath.Join("sample", "expected_output.txt")
	expectedBytes, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read expected output fixture: %v", err)
	}

	expectedLines := strings.Split(strings.TrimSpace(string(expectedBytes)), "\n")

	// Format actual output to match fixture format
	var actualLines []string
	for _, use := range uses {
		// Convert absolute path to relative path (just the filename)
		// Expected format: sample.go:line:column: name
		parts := strings.Split(use.Location, "/")
		filename := parts[len(parts)-1]
		actualLines = append(actualLines, filename+": "+use.Name)
	}

	// Compare line by line
	if len(actualLines) != len(expectedLines) {
		t.Errorf("Expected %d lines, got %d lines", len(expectedLines), len(actualLines))
		t.Logf("Expected:\n%s", strings.Join(expectedLines, "\n"))
		t.Logf("Actual:\n%s", strings.Join(actualLines, "\n"))
		return
	}

	for i, expected := range expectedLines {
		if i >= len(actualLines) {
			t.Errorf("Line %d: expected %q, but no more lines in actual output", i+1, expected)
			continue
		}
		if actualLines[i] != expected {
			t.Errorf("Line %d mismatch:\n  expected: %q\n  got:      %q", i+1, expected, actualLines[i])
		}
	}

	// If there were mismatches, show full output for debugging
	if t.Failed() {
		t.Logf("\nFull expected output:\n%s", strings.Join(expectedLines, "\n"))
		t.Logf("\nFull actual output:\n%s", strings.Join(actualLines, "\n"))
	}
}
