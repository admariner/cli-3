package role

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/mock"
	ps "github.com/planetscale/cli/internal/planetscale"
	"github.com/planetscale/cli/internal/printer"

	qt "github.com/frankban/quicktest"
)

func TestRole_ReassignCmd_NotFoundGuidance(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.Human
	p := printer.NewPrinter(&format)
	p.SetHumanOutput(&buf)

	org := "profound"
	db := "platform"
	branch := "dev"
	roleID := "pscale_api_45lwf7v03mvy.ua0vjkcqtfid"
	successor := "pscale_api_tnj3ssv8qulb"

	svc := &mock.PostgresRolesService{
		ReassignObjectsFn: func(ctx context.Context, req *ps.ReassignPostgresRoleObjectsRequest) error {
			c.Assert(req.Organization, qt.Equals, org)
			c.Assert(req.Database, qt.Equals, db)
			c.Assert(req.Branch, qt.Equals, branch)
			c.Assert(req.RoleId, qt.Equals, roleID)
			c.Assert(req.Successor, qt.Equals, successor)

			return &ps.Error{Code: ps.ErrNotFound}
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

	cmd := ReassignCmd(ch)
	cmd.SetArgs([]string{db, branch, roleID, "--successor", successor, "--force"})
	err := cmd.Execute()

	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(svc.ReassignObjectsFnInvoked, qt.IsTrue)

	msg := err.Error()
	c.Assert(strings.Contains(msg, "pscale role list platform dev --org profound"), qt.IsTrue)
	c.Assert(strings.Contains(msg, "`id` column"), qt.IsTrue)
	c.Assert(strings.Contains(msg, "`username` column"), qt.IsTrue)
}
