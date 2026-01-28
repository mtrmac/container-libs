package tlsopts

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseTLSConfig(t *testing.T) {
	for _, tc := range []struct {
		opts     []Option
		expected *tls.Config
	}{
		{[]Option{}, nil}, // no options
		{[]Option{WithMinVersion(""), WithCipherSuites("")}, nil}, // all empty
		{
			[]Option{WithMinVersion("1.2")},
			&tls.Config{MinVersion: tls.VersionTLS12},
		},
		{
			[]Option{WithCipherSuites("TLS_AES_128_GCM_SHA256")},
			&tls.Config{CipherSuites: []uint16{tls.TLS_AES_128_GCM_SHA256}},
		},
		{
			[]Option{WithMinVersion("1.3"), WithCipherSuites("TLS_AES_128_GCM_SHA256")},
			&tls.Config{MinVersion: tls.VersionTLS13, CipherSuites: []uint16{tls.TLS_AES_128_GCM_SHA256}},
		},
	} {
		config, err := BaseTLSConfig(tc.opts...)
		require.NoError(t, err)
		assert.Equal(t, tc.expected, config)
	}
}

func TestWithMinVersion(t *testing.T) {
	for _, tc := range []struct {
		version     string
		expected    *tls.Config
		expectError bool
	}{
		{"", nil, false},
		{"1.0", &tls.Config{MinVersion: tls.VersionTLS10}, false},
		{"1.1", &tls.Config{MinVersion: tls.VersionTLS11}, false},
		{"1.2", &tls.Config{MinVersion: tls.VersionTLS12}, false},
		{"1.3", &tls.Config{MinVersion: tls.VersionTLS13}, false},
		{"this_is_invalid", nil, true},
	} {
		config, err := BaseTLSConfig(WithMinVersion(tc.version))
		if tc.expectError {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, tc.expected, config)
		}
	}

	for _, tc := range []struct{ a, b string }{
		{"1.2", "1.3"},
		{"1.2", ""},
		{"", "1.2"},
	} {
		_, err := BaseTLSConfig(WithMinVersion(tc.a), WithMinVersion(tc.b))
		assert.Error(t, err)
	}
}

func TestWithCipherSuites(t *testing.T) {
	validSuites := tls.CipherSuites()
	require.NotEmpty(t, validSuites)
	suite0, suite1 := validSuites[0], validSuites[1]

	for _, tc := range []struct {
		suites      string
		expected    *tls.Config
		expectError bool
	}{
		{"", nil, false}, // empty
		{suite0.Name, &tls.Config{CipherSuites: []uint16{suite0.ID}}, false},
		{suite0.Name + "," + suite1.Name, &tls.Config{CipherSuites: []uint16{suite0.ID, suite1.ID}}, false},
		{"this_is_invalid", nil, true},
	} {
		config, err := BaseTLSConfig(WithCipherSuites(tc.suites))
		if tc.expectError {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, tc.expected, config)
		}
	}

	for _, tc := range []struct{ a, b string }{
		{suite0.Name, suite0.Name},
		{suite0.Name, ""},
		{"", suite0.Name},
	} {
		_, err := BaseTLSConfig(WithCipherSuites(tc.a), WithCipherSuites(tc.b))
		assert.Error(t, err)
	}
}

func TestWithNamedGroups(t *testing.T) {
	for _, tc := range []struct {
		groups      string
		expected    *tls.Config
		expectError bool
	}{
		{"", nil, false},
		{"x25519", &tls.Config{CurvePreferences: []tls.CurveID{tls.X25519}}, false},
		{"secp256r1", &tls.Config{CurvePreferences: []tls.CurveID{tls.CurveP256}}, false},
		{"secp384r1", &tls.Config{CurvePreferences: []tls.CurveID{tls.CurveP384}}, false},
		{"secp521r1", &tls.Config{CurvePreferences: []tls.CurveID{tls.CurveP521}}, false},
		{"x25519,secp256r1", &tls.Config{CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256}}, false},
		{"this_is_invalid", nil, true},
	} {
		config, err := BaseTLSConfig(WithNamedGroups(tc.groups))
		if tc.expectError {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, tc.expected, config)
		}
	}

	for _, tc := range []struct{ a, b string }{
		{"x25519", "secp256r1"},
		{"x25519", ""},
		{"", "x25519"},
	} {
		_, err := BaseTLSConfig(WithNamedGroups(tc.a), WithNamedGroups(tc.b))
		assert.Error(t, err)
	}
}
