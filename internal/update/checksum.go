package update

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// ExpectedSHA256 parses sha256sum.txt and returns the hash for assetName.
func ExpectedSHA256(checksums []byte, assetName string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(checksums))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[len(parts)-1]
		name = strings.TrimPrefix(name, "*")
		if name == assetName {
			return parts[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %q not found in sha256sum.txt", assetName)
}

// VerifySHA256 compares data against an expected lowercase hex digest.
func VerifySHA256(data []byte, expectedHex string) error {
	got, err := VerifySHA256Return(data)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, strings.TrimSpace(expectedHex)) {
		return fmt.Errorf("checksum mismatch: got %s", got)
	}
	return nil
}

// VerifySHA256Return returns the SHA256 hex digest of data.
func VerifySHA256Return(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// HashReader returns the SHA256 hex digest of r.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
