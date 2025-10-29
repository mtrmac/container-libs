package main

import (
	"fmt"
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

	// Check all uses (combined list)
	expectedPath := filepath.Join("sample", "expected_output.txt")
	expectedBytes, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read expected output fixture: %v", err)
	}

	expectedLines := strings.Split(strings.TrimSpace(string(expectedBytes)), "\n")

	// Format actual output to match fixture format
	var actualLines []string
	for _, use := range uses {
		actualLines = append(actualLines, use.String())
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

func TestGenerateFixtures(t *testing.T) {
	t.Skip("Helper test to generate fixture files - run manually when needed")

	samplePath, err := filepath.Abs("sample")
	if err != nil {
		t.Fatal(err)
	}

	uses, err := auditDigestUses(samplePath)
	if err != nil {
		t.Fatal(err)
	}

	// Write all uses (including ignored)
	f, err := os.Create(filepath.Join("sample", "expected_output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, u := range uses {
		fmt.Fprintln(f, u.String())
	}

	// Count ignored vs non-ignored
	var ignoredCount, reportedCount int
	for _, u := range uses {
		if u.Kind == KindIgnored {
			ignoredCount++
		} else {
			reportedCount++
		}
	}

	t.Logf("Generated %d total uses (%d reported, %d ignored)", len(uses), reportedCount, ignoredCount)
}
