package exec

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeConfigFilter(t *testing.T) {
	var discardedParsingDestination map[string]any
	unexpectedEndOfJSONInput := json.Unmarshal([]byte("{\n"), &discardedParsingDestination) // this should force the error
	fileMode := os.FileMode(0o600)
	rootUint32 := uint32(0)
	for _, tt := range []struct {
		name                   string
		contextTimeout         time.Duration
		hooks                  []spec.Hook
		input                  *spec.Spec
		expected               *spec.Spec
		expectedHookError      string
		expectedRunError       error
		expectedRunErrorString string
	}{
		{
			name: "no-op",
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", "cat"},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
		},
		{
			name: "device injection",
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", `sed 's|\("gid":0}\)|\1,{"path": "/dev/sda","type":"b","major":8,"minor":0,"fileMode":384,"uid":0,"gid":0}|'`},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{
							Path:     "/dev/fuse",
							Type:     "c",
							Major:    10,
							Minor:    229,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
					},
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{
							Path:     "/dev/fuse",
							Type:     "c",
							Major:    10,
							Minor:    229,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
						{
							Path:     "/dev/sda",
							Type:     "b",
							Major:    8,
							Minor:    0,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
					},
				},
			},
		},
		{
			name: "chaining",
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", `sed 's|\("gid":0}\)|\1,{"path": "/dev/sda","type":"b","major":8,"minor":0,"fileMode":384,"uid":0,"gid":0}|'`},
				},
				{
					Path: path,
					Args: []string{"sh", "-c", `sed 's|/dev/sda|/dev/sdb|'`},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{
							Path:     "/dev/fuse",
							Type:     "c",
							Major:    10,
							Minor:    229,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
					},
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{
							Path:     "/dev/fuse",
							Type:     "c",
							Major:    10,
							Minor:    229,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
						{
							Path:     "/dev/sdb",
							Type:     "b",
							Major:    8,
							Minor:    0,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
					},
				},
			},
		},
		{
			name:           "context timeout",
			contextTimeout: time.Duration(1) * time.Second,
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", "sleep 2"},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expectedHookError: "^executing \\[sh -c sleep 2]: signal: killed$",
			expectedRunError:  context.DeadlineExceeded,
		},
		{
			name: "hook timeout",
			hooks: []spec.Hook{
				{
					Path:    path,
					Args:    []string{"sh", "-c", "sleep 2"},
					Timeout: new(1),
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expectedHookError: "^executing \\[sh -c sleep 2]: signal: killed$",
			expectedRunError:  context.DeadlineExceeded,
		},
		{
			name: "invalid JSON",
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", "echo '{'"},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expectedRunErrorString: unexpectedEndOfJSONInput.Error(),
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.contextTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.contextTimeout)
				defer cancel()
			}
			hookErr, err := RuntimeConfigFilterWithOptions(ctx, RuntimeConfigFilterOptions{Hooks: test.hooks, Config: test.input, PostKillTimeout: DefaultPostKillTimeout})
			if test.expectedRunErrorString != "" {
				// We have to compare the error strings in that case because
				// errors.Is works differently.
				assert.Contains(t, err.Error(), test.expectedRunErrorString)
			} else {
				assert.True(t, errors.Is(err, test.expectedRunError))
			}
			if test.expectedHookError == "" {
				if hookErr != nil {
					t.Fatal(hookErr)
				}
			} else {
				assert.Regexp(t, test.expectedHookError, hookErr.Error())
			}
			assert.Equal(t, test.expected, test.input)
		})
	}
}

func TestRuntimeConfigFilterOutputRedirection(t *testing.T) {
	const preExistingStdoutContent = "existing-stdout-line\n"
	const preExistingStderrContent = "existing-stderr-line\n"

	for _, test := range []struct {
		name                string
		hookScript          string
		useStdoutAnnotation bool
		stdoutPathOverride  string
		useStderrAnnotation bool
		expectedStderr      string
		expectedErr         string
		noHooks             bool
	}{
		{
			name:       "no stderr annotation still allows hook to write to stderr",
			hookScript: "cat; printf 'stderr-content\\n' 1>&2",
		},
		{
			name:                "no hooks configured skips creating annotation files",
			useStdoutAnnotation: true,
			noHooks:             true,
		},
		{
			name:                "stdout annotation redirects output and preserves round-trip",
			hookScript:          "cat",
			useStdoutAnnotation: true,
		},
		{
			name:                "stderr annotation redirects stderr only",
			hookScript:          "printf 'stderr-content\\n' 1>&2; cat",
			useStderrAnnotation: true,
			expectedStderr:      "stderr-content\n",
		},
		{
			name:                "both annotations set redirect independently",
			hookScript:          "printf 'stderr-content\\n' 1>&2; cat",
			useStdoutAnnotation: true,
			useStderrAnnotation: true,
			expectedStderr:      "stderr-content\n",
		},
		{
			name:                "invalid stdout path returns an error",
			hookScript:          "cat",
			useStdoutAnnotation: true,
			stdoutPathOverride:  "/no/such/directory/stdout.log",
			expectedErr:         "opening stdout file",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			stdoutPath := filepath.Join(dir, "stdout.log")
			if test.stdoutPathOverride != "" {
				stdoutPath = test.stdoutPathOverride
			}
			stderrPath := filepath.Join(dir, "stderr.log")

			if test.useStdoutAnnotation && test.stdoutPathOverride == "" && !test.noHooks {
				require.NoError(t, os.WriteFile(stdoutPath, []byte(preExistingStdoutContent), 0o644))
			}
			if test.useStderrAnnotation && !test.noHooks {
				require.NoError(t, os.WriteFile(stderrPath, []byte(preExistingStderrContent), 0o644))
			}

			annotations := map[string]string{}
			if test.useStdoutAnnotation {
				annotations[annotationHookStdout] = stdoutPath
			}
			if test.useStderrAnnotation {
				annotations[annotationHookStderr] = stderrPath
			}

			input := &spec.Spec{
				Version:     "1.0.0",
				Root:        &spec.Root{Path: "rootfs"},
				Annotations: annotations,
			}

			var hooks []spec.Hook
			if !test.noHooks {
				hooks = []spec.Hook{{Path: path, Args: []string{"sh", "-c", test.hookScript}}}
			}

			expectedJSON, err := json.Marshal(input)
			require.NoError(t, err)
			inputBefore := &spec.Spec{}
			require.NoError(t, json.Unmarshal(expectedJSON, inputBefore))

			hookErr, err := RuntimeConfigFilterWithOptions(t.Context(), RuntimeConfigFilterOptions{Hooks: hooks, Config: input, PostKillTimeout: DefaultPostKillTimeout})
			if test.expectedErr != "" {
				assert.ErrorContains(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, hookErr)

			assert.Equal(t, inputBefore, input)

			if test.useStdoutAnnotation && test.noHooks {
				_, statErr := os.Stat(stdoutPath)
				assert.ErrorIs(t, statErr, os.ErrNotExist)
			} else if test.useStdoutAnnotation {
				contents, err := os.ReadFile(stdoutPath)
				require.NoError(t, err)
				assert.Equal(t, preExistingStdoutContent+string(expectedJSON), string(contents))
			}

			if test.useStderrAnnotation {
				contents, err := os.ReadFile(stderrPath)
				require.NoError(t, err)
				assert.Equal(t, preExistingStderrContent+test.expectedStderr, string(contents))
			}
		})
	}
}

func TestRuntimeConfigFilterCreatesStdoutFileWithCorrectMode(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "stdout.log")
	input := &spec.Spec{
		Version:     "1.0.0",
		Root:        &spec.Root{Path: "rootfs"},
		Annotations: map[string]string{annotationHookStdout: stdoutPath},
	}
	hooks := []spec.Hook{{Path: path, Args: []string{"sh", "-c", "cat"}}}

	hookErr, err := RuntimeConfigFilterWithOptions(t.Context(), RuntimeConfigFilterOptions{Hooks: hooks, Config: input, PostKillTimeout: DefaultPostKillTimeout})
	require.NoError(t, err)
	require.NoError(t, hookErr)

	info, err := os.Stat(stdoutPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}
