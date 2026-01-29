// Package tlscobra implements Cobra CLI flags for tlsopts.
//
// Recommended flag documentation:
//
//	--tls-min-version sets the minimum TLS version to use throughout the program.
//	If not set, defaults to a reasonable default that may change over time (depending on system’s global policy,
//	version of the program, version of the Go language, and the like).
//	Users should generally not use this option and hard-code a version unless they have a process
//	to ensure that the value will be kept up to date.
//
//
//	--tls-cipher-suites sets the allowed TLS cipher suites to use throughout the program.
//	The value is a comma-separated list of IANA TLS Cipher Suites names.
//
//	If not set, defaults to a reasonable default that may change over time (depending on system’s global policy,
//	version of the program, version of the Go language, and the like).
//
//	Warning: Almost no-one should ever use this option.  Use it only if you have a bureaucracy that requires a specific list,
//	and if you are confident that this bureaucracy will still exist, and will bring you an updated list when necessary,
//	many years from now.
//
//	Warning: The effectiveness of this option is limited by capabilities of the Go standard library;
//	e.g., as of Go 1.25, it is not possible to change which cipher suites are used in TLS 1.3.
//
//
//	--tls-named-groups sets the allowed TLS named groups to use throughout the program.
//	The value is a comma-separated list of IANA TLS Supported Groups names.
//
//	If not set, defaults to a reasonable default that may change over time (depending on system’s global policy,
//	version of the program, version of the Go language, and the like).
//
//	Warning: Almost no-one should ever use this option.  Use it only if you have a bureaucracy that requires a specific list,
//	and if you are confident that this bureaucracy will still exist, and will bring you an updated list when necessary,
//	many years from now.
package tlscobra

import (
	"crypto/tls"
	"errors"

	"github.com/spf13/pflag"
	"go.podman.io/image/v5/pkg/cli/tlsopts"
)

// TLSOptions is a set of TLS-related CLI options.
type TLSOptions struct {
	minVersion   onceStringValue
	cipherSuites onceStringValue
	namedGroups  onceStringValue
}

// TLSFlags returns a collection of CLI flags writing into TLSOptions, and the managed TLSOptions structure.
func TLSFlags() (pflag.FlagSet, *TLSOptions) {
	opts := TLSOptions{}
	fs := pflag.FlagSet{}
	fs.Var(&opts.minVersion, "tls-min-version", "Minimum TLS version")
	fs.Var(&opts.cipherSuites, "tls-cipher-suites", "Comma-separated list of TLS cipher suites")
	fs.Var(&opts.namedGroups, "tls-named-groups", "Comma-separated list of TLS named groups")

	return fs, &opts
}

// BaseTLSConfig returns a *tls.Config configured according to opts.
// If opts contains no settings (all options are empty or unset), it returns (nil, nil).
// Otherwise, the returned *tls.Config is freshly allocated and the caller can modify it as needed.
func (opts *TLSOptions) BaseTLSConfig() (*tls.Config, error) {
	// We don’t need to inspect .present, .value defaults to "".
	return tlsopts.BaseTLSConfig(
		tlsopts.WithMinVersion(opts.minVersion.value),
		tlsopts.WithCipherSuites(opts.cipherSuites.value),
		tlsopts.WithNamedGroups(opts.namedGroups.value),
	)
}

// onceStringValue is a flag.Value implementation equivalent to the one underlying pflag.String,
// except that it fails when set more than once (even to the same value).
// Compare https://github.com/spf13/pflag/issues/72 .
type onceStringValue struct {
	present bool
	value   string // Valid even if !present, defaults to "".
}

func (os *onceStringValue) String() string {
	return os.value
}

func (os *onceStringValue) Set(val string) error {
	if os.present {
		return errors.New("flag already set")
	}
	os.present = true
	os.value = val
	return nil
}

func (os *onceStringValue) Type() string {
	return "string"
}
