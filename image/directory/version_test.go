package directory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	for _, c := range []struct {
		input       string
		expected    version
		shouldError bool
	}{
		{
			input:    "Directory Transport Version: 1.1\n",
			expected: version{major: 1, minor: 1},
		},
		{
			input:    "Directory Transport Version: 1.2\n",
			expected: version{major: 1, minor: 2},
		},
		{
			input:       "Invalid prefix 1.1\n",
			shouldError: true,
		},
		{
			input:       "Directory Transport Version: 1.1",
			shouldError: true,
		},
		{
			input:       "Directory Transport Version: abc\n",
			shouldError: true,
		},
	} {
		t.Run(c.input, func(t *testing.T) {
			v, err := parseVersion(c.input)
			if c.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, c.expected, v)
			}
		})
	}
}

func TestVersionComparison(t *testing.T) {
	assert.False(t, version1_1.isGreaterThan(version1_1))
	assert.False(t, version1_1.isGreaterThan(version1_2))
	assert.True(t, version1_2.isGreaterThan(version1_1))
	assert.True(t, version{major: 2, minor: 0}.isGreaterThan(version1_2))
	assert.True(t, version{major: 1, minor: 3}.isGreaterThan(version1_2))
}
