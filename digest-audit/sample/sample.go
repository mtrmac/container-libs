package sample

import (
	"fmt"

	"github.com/opencontainers/go-digest"
)

var globalDigest digest.Digest = digest.FromString("global")

var globalDigestPtr *digest.Digest

var globalDigestArray []digest.Digest

type DigestContainer struct {
	ID     digest.Digest
	Backup *digest.Digest
	List   []*digest.Digest
}

func CoverageFunction() {
	var d1 digest.Digest = "sha256:1234567890abcdef"
	d2 := digest.FromString("test")

	var d3 digest.Digest
	d3 = d1

	_ = d1.Algorithm()

	if d1 == d2 {
		fmt.Println("equal")
	}

	_ = string(d1)

	type MyString string
	_ = MyString(d2)

	processDigest(d3, nil)

	_ = []digest.Digest{d1}

	m := map[digest.Digest]digest.Digest{d1: d2}
	_ = m[d2]

	for k, v := range m {
		_ = k
		_ = v
	}

	for _, r := range d1 {
		_ = r
	}

	ptr := &d1
	_ = *ptr

	_ = (d1)

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
