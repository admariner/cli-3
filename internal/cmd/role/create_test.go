package role

import (
	"bytes"
	"context"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/mock"
	"github.com/planetscale/cli/internal/printer"
	ps "github.com/planetscale/planetscale-go/planetscale"

	qt "github.com/frankban/quicktest"
)

func TestRole_CreateCmd(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.JSON
	p := printer.NewPrinter(&format)
	p.SetResourceOutput(&buf)

	org := "planetscale"
	db := "planetscale"
	branch := "main"
	name := "replicator"
	res := &ps.PostgresRole{ID: "role-id", Name: name, Username: "u", Password: "p"}

	svc := &mock.PostgresRolesService{
		CreateFn: func(ctx context.Context, req *ps.CreatePostgresRoleRequest) (*ps.PostgresRole, error) {
			c.Assert(req.Organization, qt.Equals, org)
			c.Assert(req.Database, qt.Equals, db)
			c.Assert(req.Branch, qt.Equals, branch)
			c.Assert(req.Name, qt.Equals, name)
			c.Assert(req.WithReplication, qt.Equals, false)
			c.Assert(req.InheritedRoles, qt.DeepEquals, []string(nil))

			return res, nil
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config: &config.Config{
			Organization: org,
		},
		Client: func() (*ps.Client, error) {
			return &ps.Client{
				PostgresRoles: svc,
			}, nil
		},
	}

	cmd := CreateCmd(ch)
	cmd.SetArgs([]string{db, branch, name})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(svc.CreateFnInvoked, qt.IsTrue)
}

func TestRole_CreateCmd_WithReplication(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.JSON
	p := printer.NewPrinter(&format)
	p.SetResourceOutput(&buf)

	org := "planetscale"
	db := "planetscale"
	branch := "main"
	name := "replicator"
	res := &ps.PostgresRole{
		ID:              "role-id",
		Name:            name,
		Username:        "u",
		Password:        "p",
		WithReplication: true,
	}

	svc := &mock.PostgresRolesService{
		CreateFn: func(ctx context.Context, req *ps.CreatePostgresRoleRequest) (*ps.PostgresRole, error) {
			c.Assert(req.Organization, qt.Equals, org)
			c.Assert(req.Database, qt.Equals, db)
			c.Assert(req.Branch, qt.Equals, branch)
			c.Assert(req.Name, qt.Equals, name)
			c.Assert(req.WithReplication, qt.IsTrue)
			c.Assert(req.InheritedRoles, qt.DeepEquals, []string{"postgres"})

			return res, nil
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config: &config.Config{
			Organization: org,
		},
		Client: func() (*ps.Client, error) {
			return &ps.Client{
				PostgresRoles: svc,
			}, nil
		},
	}

	cmd := CreateCmd(ch)
	cmd.SetArgs([]string{db, branch, name, "--with-replication", "--inherited-roles", "postgres"})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(svc.CreateFnInvoked, qt.IsTrue)
}
