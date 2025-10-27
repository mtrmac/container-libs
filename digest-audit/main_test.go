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
	uses, ignoredUses, err := auditDigestUses(samplePath)
	if err != nil {
		t.Fatalf("Failed to audit sample: %v", err)
	}

	// Check reported uses
	t.Run("ReportedUses", func(t *testing.T) {
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
	})

	// Check ignored uses
	t.Run("IgnoredUses", func(t *testing.T) {
		expectedPath := filepath.Join("sample", "expected_ignored.txt")
		expectedBytes, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("Failed to read expected ignored fixture: %v", err)
		}

		expectedLines := strings.Split(strings.TrimSpace(string(expectedBytes)), "\n")

		// Format actual output to match fixture format
		var actualLines []string
		for _, use := range ignoredUses {
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
	})
}

func TestGenerateFixtures(t *testing.T) {
	t.Skip("Helper test to generate fixture files - run manually when needed")

	samplePath, err := filepath.Abs("sample")
	if err != nil {
		t.Fatal(err)
	}

	uses, ignoredUses, err := auditDigestUses(samplePath)
	if err != nil {
		t.Fatal(err)
	}

	// Write reported uses
	f1, err := os.Create(filepath.Join("sample", "expected_output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()

	for _, u := range uses {
		fmt.Fprintln(f1, u.String())
	}

	// Write ignored uses
	f2, err := os.Create(filepath.Join("sample", "expected_ignored.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()

	for _, u := range ignoredUses {
		fmt.Fprintln(f2, u.String())
	}

	t.Logf("Generated %d reported uses and %d ignored uses", len(uses), len(ignoredUses))
}
