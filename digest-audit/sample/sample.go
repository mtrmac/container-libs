package sample

import (
	"fmt"

	"github.com/opencontainers/go-digest"
)

func ExampleFunction() {
	// Variable declaration and initialization
	var d1 digest.Digest = "sha256:1234567890abcdef"
	d2 := digest.FromString("test")

	// Assignment/copy
	d3 := d1

	// Method call on identifier
	algo := d1.Algorithm()
	_ = algo

	// Comparison of identifiers
	if d1 == d2 {
		fmt.Println("equal")
	}

	// String concatenation with conversion
	msg := "digest: " + string(d3)
	fmt.Println(msg)

	// Function parameter - identifier
	processDigest(d1)

	// Return value usage
	d4 := getDigest()
	_ = d4

	// Method call on value expression (not identifier)
	encoded := digest.FromString("inline").Encoded()
	_ = encoded

	// Comparison with value expression
	if digest.FromString("a") == digest.FromString("b") {
		fmt.Println("expressions equal")
	}

	// Function parameter - value expression
	processDigest(digest.FromString("param"))

	// Type conversion of value expression
	s := string(digest.FromString("convert"))
	_ = s

	// Method call on return value
	algo2 := getDigest().Algorithm()
	_ = algo2
}

func processDigest(d digest.Digest) {
	// Parameter usage
	encoded := d.Encoded()
	_ = encoded
}

func getDigest() digest.Digest {
	return digest.FromString("result")
}
