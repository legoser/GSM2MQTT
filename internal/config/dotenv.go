package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// parseEnvFile parses a .env formatted file into key-value pairs without external dependencies.
func parseEnvFile(path string) map[string]string {
	res := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return res
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		res[k] = v
	}
	return res
}

// loadDotEnv searches for and loads a .env file from the current directory or config directory.
func loadDotEnv(dirs ...string) map[string]string {
	candidates := []string{".env"}
	for _, d := range dirs {
		if d != "" && d != "." {
			candidates = append(candidates, filepath.Join(d, ".env"))
		}
	}

	merged := make(map[string]string)
	for _, c := range candidates {
		if m := parseEnvFile(c); len(m) > 0 {
			for k, v := range m {
				if _, exists := merged[k]; !exists {
					merged[k] = v
				}
			}
		}
	}
	return merged
}

// getHierarchicalValue returns the value for the given key variants.
// Priority: 1. OS environment (os.Getenv) -> 2. .env file -> 3. empty string.
func getHierarchicalValue(dotEnv map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	for _, k := range keys {
		if v, ok := dotEnv[k]; ok && v != "" {
			return v
		}
	}
	return ""
}
