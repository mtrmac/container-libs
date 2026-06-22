//go:build freebsd || netbsd || darwin

package archive

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// TestSyscallMode verifies that Go's os.FileMode high-order bits
// correctly translate to POSIX chmod(2) bits.
func TestSyscallMode(t *testing.T) {
	mode := syscallMode(os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0o755)
	expected := uint32(unix.S_ISUID | unix.S_ISGID | unix.S_ISVTX | 0o755)

	if mode != expected {
		t.Fatalf("expected %#o, got %#o", expected, mode)
	}
}
