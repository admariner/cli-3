package planetscale

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The API client is vendored in this package; the CLI must not depend on
// the external planetscale-go module again. See doc/api-client.md.
//
// The strings are split so this file does not match its own checks.
const (
	bannedModule = "github.com/planetscale/" + "planetscale-go"
	bannedImport = `"` + bannedModule
)

func TestNoPlanetscaleGoDependency(t *testing.T) {
	root := filepath.Join("..", "..")

	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gomod), bannedModule) {
		t.Errorf("go.mod requires %s; the API client is vendored at internal/planetscale and that module must not come back. See doc/api-client.md", bannedModule)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "testdata" || name == "bin" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), bannedImport) {
			t.Errorf("%s imports %s; use github.com/planetscale/cli/internal/planetscale instead. See doc/api-client.md", path, bannedModule)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
