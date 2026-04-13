//go:build !windows

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	jsonproxy "go.podman.io/common/pkg/json-proxy"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	sockfd := flag.Int("sockfd", -1, "socket file descriptor")
	policyPath := flag.String("policy", "", "path to policy.json (default: system default)")
	overrideArch := flag.String("override-arch", "", "override architecture for manifest list resolution")
	flag.Parse()

	if *sockfd < 0 {
		return fmt.Errorf("usage: %s --sockfd <fd> [--policy <path>] [--override-arch <arch>]", os.Args[0])
	}

	manager, err := jsonproxy.NewManager(
		jsonproxy.WithSystemContext(func() (*types.SystemContext, error) {
			sc := &types.SystemContext{}
			if *overrideArch != "" {
				sc.ArchitectureChoice = *overrideArch
			}
			return sc, nil
		}),
		jsonproxy.WithPolicyContext(func() (*signature.PolicyContext, error) {
			var policy *signature.Policy
			var err error
			if *policyPath != "" {
				policy, err = signature.NewPolicyFromFile(*policyPath)
			} else {
				policy, err = signature.DefaultPolicy(nil)
			}
			if err != nil {
				return nil, err
			}
			return signature.NewPolicyContext(policy)
		}),
	)
	if err != nil {
		return err
	}
	defer manager.Close()
	return manager.Serve(context.Background(), *sockfd)
}
