package storage

import (
	"archive/tar"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/storage/pkg/archive"
)

func TestCheckDirectory(t *testing.T) {
	vectors := []struct {
		description string
		headers     []tar.Header
		expected    []string
	}{
		{
			description: "basic",
			headers: []tar.Header{
				{Name: "a", Typeflag: tar.TypeDir},
			},
			expected: []string{
				"a/",
			},
		},
		{
			description: "whiteout",
			headers: []tar.Header{
				{Name: "a", Typeflag: tar.TypeDir},
				{Name: "a/b", Typeflag: tar.TypeDir},
				{Name: "a/b/c", Typeflag: tar.TypeReg},
				{Name: "a/b/d", Typeflag: tar.TypeReg},
				{Name: "a/b/" + archive.WhiteoutPrefix + "c", Typeflag: tar.TypeReg},
			},
			expected: []string{
				"a/",
				"a/b/",
				"a/b/d",
			},
		},
		{
			description: "opaque",
			headers: []tar.Header{
				{Name: "a", Typeflag: tar.TypeDir},
				{Name: "a/b", Typeflag: tar.TypeDir},
				{Name: "a/b/c", Typeflag: tar.TypeReg},
				{Name: "a/b/d", Typeflag: tar.TypeReg},
				{Name: "a/b/" + archive.WhiteoutOpaqueDir, Typeflag: tar.TypeReg},
			},
			expected: []string{
				"a/",
				"a/b/",
			},
		},
	}
	for i := range vectors {
		t.Run(vectors[i].description, func(t *testing.T) {
			cd := newCheckDirectoryDefaults()
			for _, hdr := range vectors[i].headers {
				cd.header(&hdr)
			}
			actual := cd.names()
			assert.ElementsMatch(t, vectors[i].expected, actual)
		})
	}
}

func TestCheckParentLayerInROStore(t *testing.T) {
	roStore := newTestStore(t, StoreOptions{})
	_, err := roStore.CreateLayer("ro-parent-layer", "", nil, "", false, nil)
	require.NoError(t, err)
	roStoreRoot := roStore.GraphRoot()
	_, err = roStore.Shutdown(false)
	require.NoError(t, err)

	// Copy the graph root to a fresh path so the global lock file cache
	// doesn't conflict (the first store registered these locks as RW).
	roStoreCopy := t.TempDir()
	require.NoError(t, os.CopyFS(roStoreCopy, os.DirFS(roStoreRoot)))

	rwStore := newTestStore(t, StoreOptions{
		GraphDriverOptions: []string{"imagestore=" + roStoreCopy},
	})
	t.Cleanup(func() { _, _ = rwStore.Shutdown(true) })

	_, err = rwStore.CreateLayer("rw-child-layer", "ro-parent-layer", nil, "", true, nil)
	require.NoError(t, err)

	report, err := rwStore.Check(nil)
	require.NoError(t, err)
	assert.Empty(t, report.Layers, "RW child layer with RO parent should not be flagged")
	assert.Empty(t, report.ROLayers, "RO parent layer should not be flagged")
}

func TestCheckDetectWriteable(t *testing.T) {
	var sawRWlayers, sawRWimages bool
	stoar, err := GetStore(StoreOptions{
		RunRoot:         t.TempDir(),
		GraphRoot:       t.TempDir(),
		GraphDriverName: "vfs",
	})
	require.NoError(t, err, "unexpected error initializing test store")
	s, ok := stoar.(*store)
	require.True(t, ok, "unexpected error making type assertion")
	_, done, err := readAllLayerStores(s, func(store roLayerStore) (struct{}, bool, error) {
		if roLayerStoreIsReallyReadWrite(store) { // implicitly checking that the type assertion in this function doesn't panic
			sawRWlayers = true
		}
		return struct{}{}, false, nil
	})
	assert.False(t, done, "unexpected error from readAllLayerStores")
	assert.NoError(t, err, "unexpected error from readAllLayerStores")
	assert.True(t, sawRWlayers, "unexpected error detecting which layer store is writeable")
	_, done, err = readAllImageStores(s, func(store roImageStore) (struct{}, bool, error) {
		if roImageStoreIsReallyReadWrite(store) { // implicitly checking that the type assertion in this function doesn't panic
			sawRWimages = true
		}
		return struct{}{}, false, nil
	})
	assert.False(t, done, "unexpected error from readAllImageStores")
	assert.NoError(t, err, "unexpected error from readAllImageStores")
	assert.True(t, sawRWimages, "unexpected error detecting which image store is writeable")
}
