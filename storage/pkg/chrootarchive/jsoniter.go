//go:build !windows && !darwin

package chrootarchive

import jsoniter "github.com/json-iterator/go"

var jsonIT = jsoniter.ConfigCompatibleWithStandardLibrary
