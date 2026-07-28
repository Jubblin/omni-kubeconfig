package update

import (
	"fmt"
	"runtime"
)

const binaryName = "omni-kubeconfig"

// AssetName returns the GoReleaser release asset filename for goos/goarch.
func AssetName(goos, goarch string) (string, error) {
	if goos == "windows" && goarch == "arm64" {
		return "", fmt.Errorf("windows/arm64 is not supported (same as omnictl)")
	}

	switch goos {
	case "darwin", "linux", "windows":
	default:
		return "", fmt.Errorf("unsupported OS %q", goos)
	}

	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture %q", goarch)
	}

	name := fmt.Sprintf("%s-%s-%s", binaryName, goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

// CurrentPlatform returns runtime.GOOS and runtime.GOARCH.
func CurrentPlatform() (goos, goarch string) {
	return runtime.GOOS, runtime.GOARCH
}

// InstalledBinaryName is the filename used when installing to a directory.
func InstalledBinaryName(goos string) string {
	if goos == "windows" {
		return binaryName + ".exe"
	}
	return binaryName
}
