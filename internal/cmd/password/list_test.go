package password

import (
	"bytes"
	"context"
	"net/url"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/mock"
	ps "github.com/planetscale/cli/internal/planetscale"
	"github.com/planetscale/cli/internal/printer"

	qt "github.com/frankban/quicktest"
)

func listQueryParam(opts []ps.ListOption, key string) string {
	lo := &ps.ListOptions{URLValues: &url.Values{}}
	for _, opt := range opts {
		_ = opt(lo)
	}
	return lo.URLValues.Get(key)
}

func TestPassword_ListCmd(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.JSON
	p := printer.NewPrinter(&format)
	p.SetResourceOutput(&buf)

	org := "planetscale"
	db := "planetscale"
	branch := "development"

	resp := []*ps.DatabaseBranchPassword{
		{Name: "foo"},
		{Name: "bar"},
	}

	svc := &mock.PasswordsService{
		ListFn: func(ctx context.Context, req *ps.ListDatabaseBranchPasswordRequest, opts ...ps.ListOption) ([]*ps.DatabaseBranchPassword, error) {
			c.Assert(req.Organization, qt.Equals, org)
			c.Assert(req.Database, qt.Equals, db)
			c.Assert(req.Branch, qt.Equals, branch)
			c.Assert(listQueryParam(opts, "q"), qt.Equals, "production")
			c.Assert(listQueryParam(opts, "status"), qt.Equals, "renewable")
			c.Assert(listQueryParam(opts, "per_page"), qt.Equals, "100")
			c.Assert(listQueryParam(opts, "page"), qt.Equals, "")

			return resp, nil
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config: &config.Config{
			Organization: org,
		},
		Client: func() (*ps.Client, error) {
			return &ps.Client{
				Passwords: svc,
			}, nil
		},
	}

	cmd := ListCmd(ch)
	cmd.SetArgs([]string{db, branch, "--name", "production", "--status", "renewable"})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(svc.ListFnInvoked, qt.IsTrue)

	passwords := []*Password{
		{
			Name: "foo",
			orig: resp[0],
		},
		{
			Name: "bar",
			orig: resp[1],
		},
	}

	c.Assert(buf.String(), qt.JSONEquals, passwords)
}

func TestPassword_ListCmdPagination(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.JSON
	p := printer.NewPrinter(&format)
	p.SetResourceOutput(&buf)

	org := "planetscale"
	db := "planetscale"
	branch := "development"

	resp := []*ps.DatabaseBranchPassword{{Name: "foo"}}

	listCalls := 0
	svc := &mock.PasswordsService{
		ListFn: func(ctx context.Context, req *ps.ListDatabaseBranchPasswordRequest, opts ...ps.ListOption) ([]*ps.DatabaseBranchPassword, error) {
			listCalls++
			c.Assert(listQueryParam(opts, "page"), qt.Equals, "2")
			c.Assert(listQueryParam(opts, "per_page"), qt.Equals, "50")
			return resp, nil
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config: &config.Config{
			Organization: org,
		},
		Client: func() (*ps.Client, error) {
			return &ps.Client{Passwords: svc}, nil
		},
	}

	cmd := ListCmd(ch)
	cmd.SetArgs([]string{db, branch, "--page", "2", "--per-page", "50"})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(listCalls, qt.Equals, 1)
	c.Assert(svc.ListFnInvoked, qt.IsTrue)
}

func TestPassword_ListCmdFilteredEmpty(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.Human
	p := printer.NewPrinter(&format)
	p.SetHumanOutput(&buf)

	svc := &mock.PasswordsService{
		ListFn: func(context.Context, *ps.ListDatabaseBranchPasswordRequest, ...ps.ListOption) ([]*ps.DatabaseBranchPassword, error) {
			return nil, nil
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config:  &config.Config{Organization: "planetscale"},
		Client: func() (*ps.Client, error) {
			return &ps.Client{Passwords: svc}, nil
		},
	}

	cmd := ListCmd(ch)
	cmd.SetArgs([]string{"planetscale", "development", "--name", "production"})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(buf.String(), qt.Contains, "match the specified filters")
	c.Assert(buf.String(), qt.Not(qt.Contains), "No passwords exist")
}
