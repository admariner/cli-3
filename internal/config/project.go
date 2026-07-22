package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// projectConfigAllowedKeys are the only keys accepted from a repo-local or
// working-directory .pscale.yml. Credentials and API endpoints must never come
// from directory-local files — a checked-in config can otherwise redirect
// requests (and the user's keyring token) to an attacker-controlled host.
var projectConfigAllowedKeys = map[string]struct{}{
	"org":      {},
	"database": {},
	"branch":   {},
}

// FilterProjectConfig keeps only allowlisted project settings from raw YAML.
// Non-allowlisted keys are returned in ignored (sorted) for caller warnings.
func FilterProjectConfig(raw map[string]interface{}) (allowed map[string]interface{}, ignored []string) {
	allowed = make(map[string]interface{})
	for k, v := range raw {
		key := strings.ToLower(strings.TrimSpace(k))
		if _, ok := projectConfigAllowedKeys[key]; ok {
			allowed[key] = v
			continue
		}
		ignored = append(ignored, k)
	}
	sort.Strings(ignored)
	return allowed, ignored
}

// MergeProjectConfig reads dir/.pscale.yml and merges only allowlisted keys into
// v. Sensitive keys such as api-url / api-token / service-token are ignored.
// Missing files are a no-op. Returns the list of ignored keys (may be empty).
func MergeProjectConfig(v *viper.Viper, dir string) (ignored []string, err error) {
	if dir == "" {
		return nil, nil
	}
	path := filepath.Join(dir, projectConfigName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(raw) == 0 {
		return nil, nil
	}

	allowed, ignored := FilterProjectConfig(raw)
	if len(allowed) == 0 {
		return ignored, nil
	}
	if err := v.MergeConfigMap(allowed); err != nil {
		return ignored, err
	}
	return ignored, nil
}

// WarnIgnoredProjectConfigKeys prints a stderr warning when a project config
// tried to set keys outside the allowlist (especially credentials / api-url).
func WarnIgnoredProjectConfigKeys(path string, ignored []string) {
	if len(ignored) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: ignoring non-allowlisted keys in %s: %s (project config may only set: org, database, branch)\n",
		path, strings.Join(ignored, ", "))
}
