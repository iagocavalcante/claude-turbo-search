package sync

import "testing"

func TestRepoName(t *testing.T) {
	cases := map[string]string{
		"git@github.com:iagocavalcante/claude-turbo-search.git": "iagocavalcante/claude-turbo-search",
		"git@github.com:foo/bar":                                "foo/bar",
		"ssh://git@github.com/foo/bar.git":                      "foo/bar",
		"ssh://git@github.com:22/foo/bar.git":                   "foo/bar",
		"https://github.com/foo/bar.git":                        "foo/bar",
		"https://github.com/foo/bar":                            "foo/bar",
		"https://user@github.com/foo/bar.git":                   "foo/bar",
		"git@gitlab.com:group/subgroup/proj.git":                "subgroup/proj", // last two segments
		"":                                                      "",
		"   ":                                                   "",
		"not-a-url":                                             "not-a-url",
		"https://example.com/":                                  "",
	}
	for in, want := range cases {
		if got := RepoName(in); got != want {
			t.Errorf("RepoName(%q) = %q, want %q", in, got, want)
		}
	}
}
