package branch

import (
	"fmt"

	"github.com/planetscale/cli/internal/cmdutil"
	ps "github.com/planetscale/cli/internal/planetscale"
	"github.com/planetscale/cli/internal/printer"
	"github.com/spf13/cobra"
)

// ResizeStatusCmd shows the most recent change request for a Postgres branch.
func ResizeStatusCmd(ch *cmdutil.Helper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <database> <branch>",
		Short: "Show the latest change request for a Postgres branch",
		Args:  cmdutil.RequiredArgs("database", "branch"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			database, branch := args[0], args[1]

			client, err := ch.Client()
			if err != nil {
				return err
			}

			end := ch.Printer.PrintProgress(fmt.Sprintf("Fetching changes for branch %s in %s...", printer.BoldBlue(branch), printer.BoldBlue(database)))
			defer end()

			changes, err := client.PostgresBranches.ListChanges(ctx, &ps.ListPostgresBranchChangesRequest{
				Organization: ch.Config.Organization,
				Database:     database,
				Branch:       branch,
			})
			if err != nil {
				switch cmdutil.ErrCode(err) {
				case ps.ErrNotFound:
					return fmt.Errorf("database %s or branch %s does not exist in organization %s", printer.BoldBlue(database), printer.BoldBlue(branch), printer.BoldBlue(ch.Config.Organization))
				default:
					return cmdutil.HandleError(err)
				}
			}
			end()

			if len(changes) == 0 {
				if ch.Printer.Format() == printer.Human {
					ch.Printer.Printf("Branch %s has no change requests.\n", printer.BoldBlue(branch))
					return nil
				}
				return ch.Printer.PrintResource([]*postgresBranchResize{})
			}

			// The API returns changes most recent first.
			return ch.Printer.PrintResource(toPostgresBranchResize(changes[0]))
		},
	}

	return cmd
}
