//go:build !windows && !darwin

package graphdriver

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.podman.io/storage/pkg/idtools"
)

func fillTestFiles(b *testing.B, path string, amount int) {
	dirCount := 0
	dir := path
	for i := range amount {
		if i%256 == 0 {
			dir = filepath.Join(path, "dir"+strconv.Itoa(dirCount))
			err := os.Mkdir(dir, 0o700)
			require.NoError(b, err)
			dirCount++
		}
		f, err := os.Create(filepath.Join(dir, strconv.Itoa(i)))
		require.NoError(b, err)
		f.Close()
	}
}

func Benchmark_LChown(b *testing.B) {
	uid := os.Getuid()
	gid := os.Getgid()
	ids := idtools.NewIDMappingsFromMaps([]idtools.IDMap{
		{
			ContainerID: uid,
			HostID:      uid,
			Size:        1,
		},
	}, []idtools.IDMap{
		{
			ContainerID: gid,
			HostID:      gid,
			Size:        1,
		},
	})

	for _, amount := range []int{1, 10, 100, 1_000, 10_000, 100_000} {
		b.Run(strconv.Itoa(amount), func(b *testing.B) {
			path := b.TempDir()
			fillTestFiles(b, path, amount)
			for b.Loop() {
				chowner := newLChowner()
				var chown fs.WalkDirFunc = func(path string, d fs.DirEntry, _ error) error {
					info, err := d.Info()
					if err != nil {
						return err
					}
					return chowner.LChown(path, info, ids, nil)
				}
				err := filepath.WalkDir(path, chown)
				require.NoError(b, err)
				// log as custom metric the number of elements in the inodes map to show the improvement best
				b.ReportMetric(float64(len(chowner.inodes)), "inodes_count/op")
			}
		})
	}
}
