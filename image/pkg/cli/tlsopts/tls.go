// Package tlsopts provides a common implementation of parsing TLS configuration options,
// to ensure consistent semantics across containers/* projects.
//
// Consider using c/image/pkg/cli/tlsopts/tlscobra instead.
package tlsopts

import (
	"crypto/tls"
	"fmt"
	"strings"
)

type optionState struct {
	anyOptionSet    bool // true if any option was set to a non-empty value
	gotMinVersion   bool
	gotCipherSuites bool
	gotNamedGroups  bool
}

// Option is a functional option for configuring TLS settings.
type Option = func(state *optionState, config *tls.Config) error

// BaseTLSConfig returns a *tls.Config configured according to opts.
// If opts contains no settings (all options are empty or unset), it returns (nil, nil).
// Otherwise, the returned *tls.Config is freshly allocated and the caller can modify it as needed.
func BaseTLSConfig(opts ...Option) (*tls.Config, error) {
	state := optionState{}
	config := tls.Config{}
	for _, opt := range opts {
		if err := opt(&state, &config); err != nil {
			return nil, err
		}
	}

	if !state.anyOptionSet {
		return nil, nil
	}

	return &config, nil
}

// tlsVersions maps TLS version strings to their crypto/tls constants.
// We could use the `tls.VersionName` names, but those are verbose and contain spaces;
// similarly the OpenShift enum values (“VersionTLS11”) are unergonomic.
var tlsVersions = map[string]uint16{
	"1.0": tls.VersionTLS10,
	"1.1": tls.VersionTLS11,
	"1.2": tls.VersionTLS12,
	"1.3": tls.VersionTLS13,
}

// WithMinVersion returns an Option that sets the minimum TLS version.
// The version string should be one of: "1.0", "1.1", "1.2", "1.3".
// If version is "", the option is treated as unset and has no effect.
// It is an error to pass this option to BaseTLSConfig more than once.
func WithMinVersion(version string) Option {
	return func(state *optionState, config *tls.Config) error {
		if state.gotMinVersion {
			return fmt.Errorf("TLS minimum version specified more than once")
		}
		state.gotMinVersion = true
		if version == "" {
			return nil
		}

		v, ok := tlsVersions[version]
		if !ok {
			return fmt.Errorf("unrecognized TLS minimum version %q", version)
		}
		config.MinVersion = v
		state.anyOptionSet = true
		return nil
	}
}

// cipherSuitesByName returns a map from cipher suite name to its ID.
func cipherSuitesByName() map[string]uint16 {
	// The Go standard library uses IANA names and already contains the mapping (for relevant values)
	// sadly we still need to turn it into a lookup map.
	suites := make(map[string]uint16)
	for _, cs := range tls.CipherSuites() {
		suites[cs.Name] = cs.ID
	}
	for _, cs := range tls.InsecureCipherSuites() {
		suites[cs.Name] = cs.ID
	}
	return suites
}

// WithCipherSuites returns an Option that sets the allowed TLS cipher suites.
// The cipherSuites string should be a comma-separated list of IANA TLS Cipher Suites names.
// If cipherSuites is "", the option is treated as unset and has no effect.
// It is an error to pass this option to BaseTLSConfig more than once.
func WithCipherSuites(cipherSuites string) Option {
	return func(state *optionState, config *tls.Config) error {
		if state.gotCipherSuites {
			return fmt.Errorf("TLS cipher suites specified more than once")
		}
		state.gotCipherSuites = true
		if cipherSuites == "" {
			return nil
		}

		suitesByName := cipherSuitesByName()

		ids := []uint16{}
		for name := range strings.SplitSeq(cipherSuites, ",") {
			id, ok := suitesByName[name]
			if !ok {
				return fmt.Errorf("unrecognized TLS cipher suite %q", name)
			}
			ids = append(ids, id)
		}

		config.CipherSuites = ids
		state.anyOptionSet = true
		return nil
	}
}

// groupsByName maps curve/group names to their tls.CurveID.
// The names match IANA TLS Supported Groups registry.
//
// Yes, the x25519 names differ in capitalization.
// Go’s tls.CurveID has a .String() method, but it
// uses the Go names.
var groupsByName = map[string]tls.CurveID{
	"secp256r1":      tls.CurveP256,
	"secp384r1":      tls.CurveP384,
	"secp521r1":      tls.CurveP521,
	"x25519":         tls.X25519,
	"X25519MLKEM768": tls.X25519MLKEM768,
}

// WithNamedGroups returns an Option that sets the allowed key exchange groups.
// The groups string should be a comma-separated list of IANA TLS Supported Groups names.
// If groups is "", the option is treated as unset and has no effect.
// It is an error to pass this option to BaseTLSConfig more than once.
func WithNamedGroups(groups string) Option {
	return func(state *optionState, config *tls.Config) error {
		if state.gotNamedGroups {
			return fmt.Errorf("TLS named groups specified more than once")
		}
		state.gotNamedGroups = true
		if groups == "" {
			return nil
		}

		ids := []tls.CurveID{}
		for name := range strings.SplitSeq(groups, ",") {
			id, ok := groupsByName[name]
			if !ok {
				return fmt.Errorf("unrecognized TLS named group %q", name)
			}
			ids = append(ids, id)
		}

		config.CurvePreferences = ids
		state.anyOptionSet = true
		return nil
	}
}
