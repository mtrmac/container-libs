package seccomp

import (
	"slices"
	"testing"
)

// The kernel reads the int arguments of socket(2) from the low 32 bits, but
// seccomp compares all 64, so a deny rule on a value has to mask the rest.
func TestDefaultProfileSocketDenyRulesMaskUpperBits(t *testing.T) {
	for _, call := range DefaultProfile().Syscalls {
		if call.Action != ActErrno || !slices.Contains(call.Names, "socket") {
			continue
		}
		for _, arg := range call.Args {
			if arg.Op != OpMaskedEqual || arg.Value != 0xffffffff {
				t.Errorf("socket deny rule %v: arg%d must use %s with mask 0xffffffff, got %s %#x",
					call.Errno, arg.Index, OpMaskedEqual, arg.Op, arg.Value)
			}
		}
	}
}
