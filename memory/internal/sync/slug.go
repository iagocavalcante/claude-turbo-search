package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

const slugLen = 12

// Slug returns a stable 12-char hex hash of the given remote URL.
// Use for naming a repo's storage on the web service without exposing the URL.
func Slug(remoteURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(remoteURL)))
	return hex.EncodeToString(sum[:])[:slugLen]
}

// OriginURL runs `git -C <repoRoot> remote get-url origin` and returns the result.
func OriginURL(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
