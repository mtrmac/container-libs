package sample

import (
	"fmt"

	"github.com/opencontainers/go-digest"
)

var globalDigest digest.Digest = digest.FromString("global")

var globalDigestPtr *digest.Digest

type DigestContainer struct {
	ID     digest.Digest
	Backup *digest.Digest
}

func CoverageFunction() {
	var d1 digest.Digest = "sha256:1234567890abcdef"

	d2 := digest.FromString("test")

	var d3 digest.Digest
	d3 = d1
	_ = d3

	algo := d1.Algorithm()
	_ = algo

	if d1 == d2 {
		fmt.Println("equal")
	}

	s := string(d1)
	_ = s

	type MyString string
	s2 := MyString(d2)
	_ = s2

	processDigest(d2, nil)

	digests := []digest.Digest{d1}
	_ = digests

	digestKeyMap := map[digest.Digest]string{d1: "value"}

	_ = digestKeyMap[d2]

	for _, d := range digests {
		_ = d
	}

	ptr := &d1
	_ = ptr

	derefDigest := *ptr
	_ = derefDigest

	switch d1 {
	case digest.FromString("test"):
		fmt.Println("test")
	}
}

func processDigest(d digest.Digest, ptr *digest.Digest) {
	_ = d
	_ = ptr
}

func getDigest() digest.Digest {
	return digest.FromString("result")
}

func (dc *DigestContainer) GetID() digest.Digest {
	return dc.ID
}
