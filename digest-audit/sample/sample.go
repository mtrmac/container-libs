package sample

import (
	"fmt"

	"github.com/opencontainers/go-digest"
)

// Test all reported use cases
func ExampleFunction() {
	// Variable declaration and initialization (identifiers)
	var d1 digest.Digest = "sha256:1234567890abcdef"
	d2 := digest.FromString("test")

	// Assignment/copy (ignored - RHS of assignment)
	d3 := d1
	_ = d3

	// Method call on identifier (selector)
	algo := d1.Algorithm()
	_ = algo

	// Comparison of identifiers (binary-op)
	if d1 == d2 {
		fmt.Println("equal")
	}

	// String concatenation with conversion (type-cast)
	msg := "digest: " + string(d3)
	fmt.Println(msg)

	// Function parameter - identifier (call-arg)
	processDigest(d1)

	// Return value usage (ignored - RHS of :=)
	d4 := getDigest()
	_ = d4

	// Method call on value expression (selector)
	encoded := digest.FromString("inline").Encoded()
	_ = encoded

	// Comparison with value expression (binary-op)
	if digest.FromString("a") == digest.FromString("b") {
		fmt.Println("expressions equal")
	}

	// Function parameter - value expression (call-arg)
	processDigest(digest.FromString("param"))

	// Type conversion of value expression (type-cast)
	s := string(digest.FromString("convert"))
	_ = s

	// Method call on return value (selector)
	algo2 := getDigest().Algorithm()
	_ = algo2

	// Composite literal with digest (composite-lit)
	digests := []digest.Digest{d1, d2}
	_ = digests

	// Map with digest keys and values (key-value)
	digestMap := map[string]digest.Digest{
		"key1": d1,
		"key2": d2,
	}
	_ = digestMap

	// Array index with digest (index)
	idx := digestMap["key1"]
	_ = idx

	// Range over digests (range-var)
	for _, d := range digests {
		fmt.Println(d)
	}

	// Pointer operations (deref)
	dPtr := &d1
	derefDigest := *dPtr
	_ = derefDigest

	// Switch with case values (case-value)
	switch d1 {
	case digest.FromString("test"):
		fmt.Println("test")
	case digest.FromString("other"):
		fmt.Println("other")
	}

	// Unary operations (&) - unary-op
	ptr := &d2
	_ = ptr

	// Binary != operation
	if d1 != d2 {
		fmt.Println("not equal")
	}
}

// Function with digest parameter (identifier in Field)
func processDigest(d digest.Digest) {
	// Parameter usage (selector)
	encoded := d.Encoded()
	_ = encoded
}

// Function returning digest (ignored - return statement)
func getDigest() digest.Digest {
	return digest.FromString("result")
}

// Function with pointer parameter (type expression in Field - ignored)
func processPtrDigest(d *digest.Digest) {
	if d != nil {
		fmt.Println(*d)
	}
}

// Variable with pointer type (type expression in ValueSpec - ignored)
var globalDigestPtr *digest.Digest

// Variable with initializer (ignored - initializer in ValueSpec)
var globalDigest digest.Digest = digest.FromString("global")

// Struct with digest field (identifier in Field)
type DigestContainer struct {
	ID     digest.Digest
	Name   string
	Backup *digest.Digest
}

// Function demonstrating field access (field-access)
func (dc *DigestContainer) GetID() digest.Digest {
	// Field access - the field itself is a digest
	return dc.ID
}

// Switch statement on digest tag (ignored - switch tag)
func switchOnDigest(d digest.Digest) {
	switch d {
	case digest.FromString("a"):
		fmt.Println("a")
	default:
		fmt.Println("other")
	}
}

// Multiple return values (ignored - return statement)
func getMultipleDigests() (digest.Digest, digest.Digest) {
	return digest.FromString("first"), digest.FromString("second")
}

// Assignment to existing variable (ignored - RHS of assignment)
func reassignDigest() {
	var d digest.Digest
	d = digest.FromString("reassigned")
	_ = d
}

// Variadic function with digest parameters
func processVariadicDigests(prefix string, digests ...digest.Digest) {
	for _, d := range digests {
		fmt.Println(prefix, d)
	}
}

// Function with unnamed parameters
func processUnnamedDigest(digest.Digest) {
	// No implementation needed for testing
}

// Test variadic and unnamed parameter calls
func testEdgeCases() {
	d1 := digest.FromString("one")
	d2 := digest.FromString("two")

	// Variadic call (call-arg)
	processVariadicDigests("digests:", d1, d2)

	// Unnamed parameter call (call-arg)
	processUnnamedDigest(d1)

	// Pointer digest type
	var ptrDigest *digest.Digest
	if d1.Validate() == nil {
		ptrDigest = &d1
	}
	if ptrDigest != nil {
		processPtrDigest(ptrDigest)
	}
}
