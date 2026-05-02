package entity

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	filePattern    = regexp.MustCompile(`[a-zA-Z0-9_./-]+\.[a-zA-Z]{1,6}`)
	conceptPattern = regexp.MustCompile(`\b[A-Z][a-z]+(?:[A-Z][a-z]+)+\b`)
	packagePattern = regexp.MustCompile(`\b[a-z][a-z0-9]+-[a-z][a-z0-9-]+\b`)
)

func ParseFilesJSON(filesJSON string) []string {
	filesJSON = strings.TrimSpace(filesJSON)
	if filesJSON == "" || filesJSON == "[]" {
		return nil
	}
	var files []string
	if err := json.Unmarshal([]byte(filesJSON), &files); err == nil {
		return files
	}
	fallback := strings.Trim(filesJSON, "[]")
	if strings.TrimSpace(fallback) == "" {
		return nil
	}
	parts := strings.Split(fallback, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.Trim(p, `"`))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func Extract(text, filesJSON string) (files []string, concepts []string, packages []string) {
	seenFiles := map[string]bool{}
	for _, f := range ParseFilesJSON(filesJSON) {
		f = strings.TrimSpace(f)
		if f != "" && !seenFiles[f] {
			files = append(files, f)
			seenFiles[f] = true
		}
	}
	for _, m := range filePattern.FindAllString(text, -1) {
		if strings.Contains(m, "/") && !seenFiles[m] {
			files = append(files, m)
			seenFiles[m] = true
		}
	}

	seenConcepts := map[string]bool{}
	for _, c := range conceptPattern.FindAllString(text, -1) {
		if !seenConcepts[c] {
			concepts = append(concepts, c)
			seenConcepts[c] = true
		}
	}

	seenPackages := map[string]bool{}
	for _, p := range packagePattern.FindAllString(text, -1) {
		if !seenPackages[p] {
			packages = append(packages, p)
			seenPackages[p] = true
		}
	}

	return files, concepts, packages
}
