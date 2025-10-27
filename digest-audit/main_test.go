package main

import (
	"os"
	"path/filepath"
	"sort"
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

	// Expected uses in sample.go - we look for identifier references
	// The specific line/column numbers will depend on the exact formatting
	expectedCount := map[string]int{
		"d1": 5, // declaration, assignment to d3, method call, comparison, function arg
		"d2": 2, // declaration, comparison
		"d3": 2, // declaration, string conversion
		"d4": 1, // declaration (from getDigest return)
		"d":  2, // parameter, method call receiver
	}

	// Count actual uses by identifier name
	actualCount := make(map[string]int)
	for _, use := range uses {
		actualCount[use.Name]++
	}

	// Verify we found the expected identifiers with reasonable counts
	for name, minCount := range expectedCount {
		if actualCount[name] < minCount {
			t.Errorf("Expected at least %d uses of %q, got %d", minCount, name, actualCount[name])
			// Show what we found
			for _, use := range uses {
				if use.Name == name {
					t.Logf("  Found: %s", use.Location)
				}
			}
		}
	}

	// Verify all uses have proper location format
	for _, use := range uses {
		if !strings.Contains(use.Location, "sample.go:") {
			t.Errorf("Use location doesn't contain sample.go: %s", use.Location)
		}
		if !strings.Contains(use.Location, ":") {
			t.Errorf("Use location doesn't have proper format: %s", use.Location)
		}
	}

	// Verify we can parse the location
	if len(uses) > 0 {
		parts := strings.Split(uses[0].Location, ":")
		if len(parts) < 2 {
			t.Errorf("Location format incorrect: %s", uses[0].Location)
		}
	}
}

func TestVSCodeFormat(t *testing.T) {
	// Use the sample directory which already has proper setup
	samplePath, err := filepath.Abs("sample")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	uses, err := auditDigestUses(samplePath)
	if err != nil {
		t.Fatalf("Failed to audit sample: %v", err)
	}

	if len(uses) == 0 {
		t.Fatal("Expected to find at least one use")
	}

	// Verify format is compatible with VS Code (file:line:column format)
	for _, use := range uses {
		// Should be able to split on ':'
		parts := strings.Split(use.Location, ":")
		if len(parts) < 3 {
			t.Errorf("Location should have at least file:line:column format, got: %s", use.Location)
		}

		// First part should be a file path
		if !strings.HasSuffix(parts[0], ".go") {
			t.Errorf("First part should be a .go file, got: %s", parts[0])
		}
	}
}

func TestMultipleFiles(t *testing.T) {
	// Test on the actual repository
	repoRoot := filepath.Join("..", "storage")

	// Check if storage directory exists
	if _, err := os.Stat(repoRoot); os.IsNotExist(err) {
		t.Skip("Storage directory not found, skipping integration test")
	}

	uses, err := auditDigestUses(repoRoot)
	if err != nil {
		t.Fatalf("Failed to audit storage: %v", err)
	}

	// Storage should have many digest uses
	if len(uses) < 5 {
		t.Logf("Found %d uses in storage (showing first 10):", len(uses))
		for i, use := range uses {
			if i >= 10 {
				break
			}
			t.Logf("  %s: %s", use.Location, use.Name)
		}
		t.Errorf("Expected more digest uses in storage directory")
	}

	// Verify all locations are valid
	for _, use := range uses {
		if use.Location == "" {
			t.Error("Found use with empty location")
		}
		if use.Name == "" {
			t.Error("Found use with empty name")
		}
	}
}

// TestOutputFormat verifies the output can be sorted and formatted properly
func TestOutputFormat(t *testing.T) {
	uses := []DigestUse{
		{Location: "file.go:10:5", Name: "d1", Kind: "identifier"},
		{Location: "file.go:5:3", Name: "d2", Kind: "identifier"},
		{Location: "file.go:15:7", Name: "d3", Kind: "identifier"},
	}

	// Should be sortable
	sort.Slice(uses, func(i, j int) bool {
		return uses[i].Location < uses[j].Location
	})

	// After sorting, should be in lexicographic order
	// Note: "file.go:10:5" comes before "file.go:5:3" lexicographically
	if uses[0].Location != "file.go:10:5" {
		t.Errorf("Expected first element after sort to be file.go:10:5, got %s", uses[0].Location)
	}
	if uses[1].Location != "file.go:15:7" {
		t.Errorf("Expected second element after sort to be file.go:15:7, got %s", uses[1].Location)
	}
	if uses[2].Location != "file.go:5:3" {
		t.Errorf("Expected third element after sort to be file.go:5:3, got %s", uses[2].Location)
	}
}
