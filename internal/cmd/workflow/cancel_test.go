package workflow

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/mock"
	"github.com/planetscale/cli/internal/printer"

	qt "github.com/frankban/quicktest"
	ps "github.com/planetscale/planetscale-go/planetscale"
)

func TestWorkflow_CancelCmd(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.JSON
	p := printer.NewPrinter(&format)
	p.SetResourceOutput(&buf)

	org := "planetscale"
	db := "planetscale"
	workflowNumber := uint64(123)

	createdAt := time.Now()
	cancelledAt := time.Now()

	// Create expected workflow response
	expectedWorkflow := &ps.Workflow{
		ID:          "workflow1",
		Number:      workflowNumber,
		Name:        "test-workflow",
		State:       "cancelled",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
		CancelledAt: &cancelledAt,
		Tables:      []*ps.WorkflowTable{{Name: "table1"}, {Name: "table2"}},
		SourceKeyspace: ps.Keyspace{
			Name: "source_ks",
		},
		TargetKeyspace: ps.Keyspace{
			Name: "target_ks",
		},
		Branch: ps.DatabaseBranch{
			Name: "main",
		},
		Actor: ps.Actor{
			Name: "test-user",
		},
		CancelledBy: &ps.Actor{
			Name: "test-user",
		},
	}

	// Mock the workflow service
	svc := &mock.WorkflowsService{
		CancelFn: func(ctx context.Context, req *ps.CancelWorkflowRequest) (*ps.Workflow, error) {
			c.Assert(req.Database, qt.Equals, db)
			c.Assert(req.Organization, qt.Equals, org)
			c.Assert(req.WorkflowNumber, qt.Equals, workflowNumber)

			return expectedWorkflow, nil
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config: &config.Config{
			Organization: org,
		},
		Client: func() (*ps.Client, error) {
			return &ps.Client{
				Workflows: svc,
			}, nil
		},
	}

	cmd := CancelCmd(ch)
	cmd.SetArgs([]string{db, "123", "--force"})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(svc.CancelFnInvoked, qt.IsTrue)
	c.Assert(buf.String(), qt.JSONEquals, expectedWorkflow)
}

func TestWorkflow_CancelCmd_Error(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	format := printer.Human
	p := printer.NewPrinter(&format)
	p.SetResourceOutput(&buf)

	org := "planetscale"
	db := "planetscale"

	// Mock the workflow service to return an error
	svc := &mock.WorkflowsService{
		CancelFn: func(ctx context.Context, req *ps.CancelWorkflowRequest) (*ps.Workflow, error) {
			return nil, &ps.Error{
				Code: ps.ErrNotFound,
			}
		},
	}

	ch := &cmdutil.Helper{
		Printer: p,
		Config: &config.Config{
			Organization: org,
		},
		Client: func() (*ps.Client, error) {
			return &ps.Client{
				Workflows: svc,
			}, nil
		},
	}

	cmd := CancelCmd(ch)
	cmd.SetArgs([]string{db, "123", "--force"})
	err := cmd.Execute()

	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(svc.CancelFnInvoked, qt.IsTrue)
}
