// Package version holds semantic version metadata for omni-kubeconfig.
// Values are overridden at link time when built via Makefile or release scripts.
package version

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
)

// Version is the semantic version (without a "v" prefix), e.g. 0.1.0.
var Version = "0.1.0"

// Commit is the short git revision (optional).
var Commit = ""

// Date is the UTC build timestamp in RFC3339 form (optional).
var Date = ""

// String returns the user-facing version string for --version.
func String() string {
	v := Version
	if Commit != "" {
		v += "+" + Commit
	}
	if Date != "" {
		v += " (built " + Date + ")"
	}
	return v
}

// OmniTag returns the version string passed to the Omni client library (omnictl-style).
func OmniTag() string {
	if strings.HasPrefix(Version, "v") {
		return Version
	}
	return "v" + Version
}

// Parse returns the parsed semantic version (strict).
func Parse() (semver.Version, error) {
	return semver.Parse(Version)
}

// ParseTolerant parses Version, accepting common git-describe forms (e.g. 0.1.0-3-gabc1234).
func ParseTolerant() (semver.Version, error) {
	return semver.ParseTolerant(Version)
}

// Validate reports whether Version is a usable semver string.
func Validate() error {
	_, err := ParseTolerant()
	if err != nil {
		return fmt.Errorf("invalid version %q: %w", Version, err)
	}
	return nil
}
