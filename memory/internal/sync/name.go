package sync

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

// RepoName derives a human-readable "org/repo" label from a git remote URL.
// Returns "" when the URL doesn't match any known form.
//
// Supported forms:
//   - git@github.com:org/repo.git
//   - git@gitlab.com:group/sub/repo.git
//   - ssh://git@github.com/org/repo.git
//   - https://github.com/org/repo.git
//   - https://user@github.com/org/repo.git
func RepoName(remoteURL string) string {
	s := strings.TrimSpace(remoteURL)
	if s == "" {
		return ""
	}

	// scp-like: git@host:org/repo[.git]
	if scpRE.MatchString(s) {
		parts := scpRE.FindStringSubmatch(s)
		return cleanRepoPath(parts[3])
	}

	// Otherwise try url.Parse — handles ssh://, https://, http://.
	u, err := url.Parse(s)
	if err != nil || u.Path == "" {
		return ""
	}
	return cleanRepoPath(u.Path)
}

// scpRE matches the SCP-like form: user@host:path.
var scpRE = regexp.MustCompile(`^([\w.-]+)@([\w.-]+):(.+)$`)

func cleanRepoPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	p = strings.TrimSuffix(p, ".git")
	// Keep the last two segments: org/repo. Some hosts use deeper paths
	// (gitlab subgroups, gerrit projects) — drop the prefix to keep names short.
	parts := strings.Split(p, "/")
	if len(parts) >= 2 {
		return path.Join(parts[len(parts)-2], parts[len(parts)-1])
	}
	if len(parts) == 1 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
