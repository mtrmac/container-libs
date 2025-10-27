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

	// Method call
	algo := d1.Algorithm()
	_ = algo

	// Comparison
	if d1 == d2 {
		fmt.Println("equal")
	}

	// String concatenation
	msg := "digest: " + string(d3)
	fmt.Println(msg)

	// Function parameter
	processDigest(d1)

	// Return value usage
	d4 := getDigest()
	_ = d4
}

func processDigest(d digest.Digest) {
	// Parameter usage
	encoded := d.Encoded()
	_ = encoded
}

func getDigest() digest.Digest {
	return digest.FromString("result")
}
