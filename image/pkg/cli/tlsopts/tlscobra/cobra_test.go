package tlscobra

import (
	"crypto/tls"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSFlags(t *testing.T) {
	for _, tc := range []struct {
		args        []string
		expected    *tls.Config
		expectError bool
	}{
		{[]string{}, nil, false}, // no options
		// Each option works
		{[]string{"--tls-min-version", "1.2"}, &tls.Config{MinVersion: tls.VersionTLS12}, false},
		{[]string{"--tls-cipher-suites", "TLS_AES_128_GCM_SHA256"}, &tls.Config{CipherSuites: []uint16{tls.TLS_AES_128_GCM_SHA256}}, false},
		{[]string{"--tls-named-groups", "secp256r1"}, &tls.Config{CurvePreferences: []tls.CurveID{tls.CurveP256}}, false},
		// A smoke-test of all options together
		{
			[]string{"--tls-min-version", "1.2", "--tls-cipher-suites", "TLS_AES_128_GCM_SHA256", "--tls-named-groups", "secp256r1"},
			&tls.Config{MinVersion: tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_AES_128_GCM_SHA256}, CurvePreferences: []tls.CurveID{tls.CurveP256}},
			false,
		},
		// Duplicate options are rejected.
		{[]string{"--tls-min-version", "1.2", "--tls-min-version", "1.2"}, nil, true},
		{[]string{"--tls-cipher-suites", "TLS_AES_128_GCM_SHA256", "--tls-cipher-suites", "TLS_AES_128_GCM_SHA256"}, nil, true},
		{[]string{"--tls-named-groups", "secp256r1", "--tls-named-groups", "secp256r1"}, nil, true},
		// A smoke-test of error handling; this is tested in detail in tlsopts.
		{[]string{"--tls-min-version", "this_is_invalid"}, nil, true},
	} {
		fs, opts := TLSFlags()
		actionRun := false
		var useError error
		app := cobra.Command{
			RunE: func(_ *cobra.Command, args []string) error {
				config, err := opts.BaseTLSConfig()
				useError = err
				if err == nil {
					assert.Equal(t, tc.expected, config, tc.args)
				}
				actionRun = true
				return nil
			},
		}
		app.Flags().AddFlagSet(&fs)
		app.SetOut(io.Discard)
		app.SetErr(io.Discard)
		app.SetArgs(tc.args)
		topErr := app.Execute()
		if !tc.expectError {
			assert.True(t, actionRun)
			require.NoError(t, topErr, tc.args)
			require.NoError(t, useError, tc.args)
		} else {
			assert.True(t, topErr != nil || useError != nil)
		}
	}
}
