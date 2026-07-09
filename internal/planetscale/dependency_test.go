package planetscale

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The API client is vendored in this package; the CLI must not depend on
// the external planetscale-go module again. See doc/api-client.md.
const bannedModule = "github.com/planetscale/planetscale-go"

func TestNoPlanetscaleGoDependency(t *testing.T) {
	root := filepath.Join("..", "..")

	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gomod), bannedModule) {
		t.Errorf("go.mod requires %s; the API client is vendored at internal/planetscale and that module must not come back. See doc/api-client.md", bannedModule)
	}

	fset := token.NewFileSet()
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
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if importPath == bannedModule || strings.HasPrefix(importPath, bannedModule+"/") {
				t.Errorf("%s imports %s; use github.com/planetscale/cli/internal/planetscale instead. See doc/api-client.md", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
