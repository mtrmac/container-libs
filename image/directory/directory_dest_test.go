package directory

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/pkg/blobinfocache/memory"
	"go.podman.io/image/v5/types"
)

func TestVersionAssignment(t *testing.T) {
	for _, contentType := range []struct {
		name string
		put  func(*testing.T, types.ImageDestination, digest.Algorithm, int, types.BlobInfoCache)
	}{
		{
			name: "blob",
			put: func(t *testing.T, dest types.ImageDestination, algo digest.Algorithm, i int, cache types.BlobInfoCache) {
				blobData := []byte("test-blob-" + algo.String() + "-" + string(rune(i)))
				var blobDigest digest.Digest
				if algo == digest.SHA256 {
					blobDigest = ""
				} else {
					blobDigest = algo.FromBytes(blobData)
				}
				_, err := dest.PutBlob(context.Background(), bytes.NewReader(blobData), types.BlobInfo{Digest: blobDigest, Size: int64(len(blobData))}, cache, false)
				require.NoError(t, err)
			},
		},
		{
			name: "manifest",
			put: func(t *testing.T, dest types.ImageDestination, algo digest.Algorithm, i int, cache types.BlobInfoCache) {
				manifestData := []byte("test-manifest-" + algo.String() + "-" + string(rune(i)))
				instanceDigest := algo.FromBytes(manifestData)
				err := dest.PutManifest(context.Background(), manifestData, &instanceDigest)
				require.NoError(t, err)
			},
		},
		{
			name: "signature",
			put: func(t *testing.T, dest types.ImageDestination, algo digest.Algorithm, i int, cache types.BlobInfoCache) {
				manifestData := []byte("test-manifest-" + algo.String() + "-" + string(rune(i)))
				instanceDigest := algo.FromBytes(manifestData)
				// These signatures are completely invalid; start with 0xA3 just to be minimally plausible to signature.FromBlob.
				signatures := [][]byte{[]byte("\xA3sig" + algo.String() + "-" + string(rune(i)))}
				err := dest.PutSignatures(context.Background(), signatures, &instanceDigest)
				require.NoError(t, err)
			},
		},
	} {
		for _, c := range []struct {
			name            string
			algorithms      []digest.Algorithm
			expectedVersion version
		}{
			{
				name:            "SHA256 only gets version 1.1",
				algorithms:      []digest.Algorithm{digest.SHA256},
				expectedVersion: version1_1,
			},
			{
				name:            "SHA512 only gets version 1.2",
				algorithms:      []digest.Algorithm{digest.SHA512},
				expectedVersion: version1_2,
			},
			{
				name:            "Mixed SHA256 and SHA512 gets version 1.2",
				algorithms:      []digest.Algorithm{digest.SHA256, digest.SHA512},
				expectedVersion: version1_2,
			},
		} {
			t.Run(contentType.name+": "+c.name, func(t *testing.T) {
				ref, tmpDir := refToTempDir(t)
				cache := memory.New()

				dest, err := ref.NewImageDestination(context.Background(), nil)
				require.NoError(t, err)
				defer dest.Close()

				for i, algo := range c.algorithms {
					contentType.put(t, dest, algo, i, cache)
				}

				err = dest.Commit(context.Background(), nil)
				require.NoError(t, err)

				versionBytes, err := os.ReadFile(filepath.Join(tmpDir, "version"))
				require.NoError(t, err)
				assert.Equal(t, c.expectedVersion.String(), string(versionBytes))
			})
		}
	}
}
