package runtime

import (
	"os"
	"path/filepath"
)

func ResolveConfigPath() string {
	return filepath.Join(baseDir(), "config.yaml")
}

func ResolveEventsPath() string {
	if p := resolveFile("events.json"); p != "" {
		return p
	}
	return filepath.Join(baseDir(), "events.json")
}

func ResolveDataDBPath() string {
	return filepath.Join(baseDir(), "data.db")
}

func ResolveArchiveDBPath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(baseDir(), name)
}

func resolveFile(name string) string {
	for _, p := range resolveFileCandidates(name) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func resolveFileCandidates(name string) []string {
	seen := map[string]struct{}{}
	list := make([]string, 0, 32)
	addCandidates(osGetwd(), name, seen, &list)
	addCandidates(baseDir(), name, seen, &list)
	return list
}

func addCandidates(startDir, name string, seen map[string]struct{}, list *[]string) {
	dir := startDir
	for i := 0; i < 10; i++ {
		if dir == "" {
			return
		}
		candidate := filepath.Join(dir, name)
		if _, ok := seen[candidate]; !ok {
			seen[candidate] = struct{}{}
			if _, err := os.Stat(candidate); err == nil {
				*list = append(*list, candidate)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func baseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return osGetwd()
	}
	return filepath.Dir(exe)
}

func osGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
