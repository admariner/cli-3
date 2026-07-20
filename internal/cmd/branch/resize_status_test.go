package branch

import (
	"bytes"
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/mock"
	ps "github.com/planetscale/cli/internal/planetscale"
	"github.com/planetscale/cli/internal/printer"
)

func TestBranch_ResizeStatusCmd(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.JSON
	p := printer.NewPrinter(&format)
	p.SetResourceOutput(&buf)

	org := "planetscale"
	db := "postgres-db"
	branch := "main"

	pgSvc := &mock.PostgresBranchesService{
		ListChangesFn: func(ctx context.Context, req *ps.ListPostgresBranchChangesRequest) ([]*ps.PostgresBranchClusterResizeRequest, error) {
			c.Assert(req.Organization, qt.Equals, org)
			c.Assert(req.Database, qt.Equals, db)
			c.Assert(req.Branch, qt.Equals, branch)

			return []*ps.PostgresBranchClusterResizeRequest{
				{ID: "change-2", State: "resizing"},
				{ID: "change-1", State: "completed"},
			}, nil
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config: &config.Config{
			Organization: org,
		},
		Client: func() (*ps.Client, error) {
			return &ps.Client{PostgresBranches: pgSvc}, nil
		},
	}

	cmd := ResizeStatusCmd(ch)
	cmd.SetArgs([]string{db, branch})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(pgSvc.ListChangesFnInvoked, qt.IsTrue)
	// Only the most recent change is printed.
	c.Assert(buf.String(), qt.Contains, `"id": "change-2"`)
	c.Assert(buf.String(), qt.Not(qt.Contains), `"id": "change-1"`)
}

func TestBranch_ResizeStatusCmd_NoChanges(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.JSON
	p := printer.NewPrinter(&format)
	p.SetResourceOutput(&buf)

	pgSvc := &mock.PostgresBranchesService{
		ListChangesFn: func(ctx context.Context, req *ps.ListPostgresBranchChangesRequest) ([]*ps.PostgresBranchClusterResizeRequest, error) {
			return nil, nil
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config: &config.Config{
			Organization: "planetscale",
		},
		Client: func() (*ps.Client, error) {
			return &ps.Client{PostgresBranches: pgSvc}, nil
		},
	}

	cmd := ResizeStatusCmd(ch)
	cmd.SetArgs([]string{"postgres-db", "main"})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(buf.String(), qt.Contains, "[]")
}
