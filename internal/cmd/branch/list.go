package branch

import (
	"fmt"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/planetscale"
	"github.com/planetscale/cli/internal/printer"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

// ListCmd encapsulates the command for listing branches for a database.
func ListCmd(ch *cmdutil.Helper) *cobra.Command {
	var flags struct {
		page    int
		perPage int
	}

	cmd := &cobra.Command{
		Use:     "list <database>",
		Short:   "List all branches of a database",
		Args:    cmdutil.RequiredArgs("database"),
		Aliases: []string{"ls"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			return cmdutil.DatabaseCompletionFunc(ch, cmd, args, toComplete)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			database := args[0]

			web, err := cmd.Flags().GetBool("web")
			if err != nil {
				return err
			}

			if web {
				ch.Printer.Println("🌐  Redirecting you to your branches in your web browser.")
				err := browser.OpenURL(fmt.Sprintf("%s/%s/%s/branches", cmdutil.ApplicationURL, ch.Config.Organization, database))
				if err != nil {
					return err
				}
				return nil
			}

			client, err := ch.Client()
			if err != nil {
				return err
			}

			end := ch.Printer.PrintProgress(fmt.Sprintf("Fetching branches for %s", printer.BoldBlue(database)))
			defer end()

			db, err := client.Databases.Get(ctx, &planetscale.GetDatabaseRequest{
				Organization: ch.Config.Organization,
				Database:     database,
			})
			if err != nil {
				switch cmdutil.ErrCode(err) {
				case planetscale.ErrNotFound:
					return cmdutil.HandleNotFoundWithServiceTokenCheck(
						ctx, cmd, ch.Config, ch.Client, err, "read_branch",
						"database %s does not exist in organization %s",
						printer.BoldBlue(database), printer.BoldBlue(ch.Config.Organization))
				default:
					return cmdutil.HandleError(err)
				}
			}

			if db.Kind == "mysql" {
				branches, err := client.DatabaseBranches.List(ctx, &planetscale.ListDatabaseBranchesRequest{
					Organization: ch.Config.Organization,
					Database:     database,
				}, planetscale.WithPage(flags.page), planetscale.WithPerPage(flags.perPage))
				if err != nil {
					switch cmdutil.ErrCode(err) {
					case planetscale.ErrNotFound:
						return cmdutil.HandleNotFoundWithServiceTokenCheck(
							ctx, cmd, ch.Config, ch.Client, err, "read_branch",
							"database %s does not exist in organization %s",
							printer.BoldBlue(database), printer.BoldBlue(ch.Config.Organization))
					default:
						return cmdutil.HandleError(err)
					}
				}

				end()

				if len(branches) == 0 && ch.Printer.Format() == printer.Human {
					if flags.page == 0 {
						ch.Printer.Printf("No branches exist in %s.\n", printer.BoldBlue(database))
					} else {
						ch.Printer.Println("No branches found on this page.")
					}
					return nil
				}

				return ch.Printer.PrintResource(toDatabaseBranches(branches))
			}

			branches, err := client.PostgresBranches.List(ctx, &planetscale.ListPostgresBranchesRequest{
				Organization: ch.Config.Organization,
				Database:     database,
			}, planetscale.WithPage(flags.page), planetscale.WithPerPage(flags.perPage))
			if err != nil {
				switch cmdutil.ErrCode(err) {
				case planetscale.ErrNotFound:
					return cmdutil.HandleNotFoundWithServiceTokenCheck(
						ctx, cmd, ch.Config, ch.Client, err, "read_branch",
						"database %s does not exist in organization %s",
						printer.BoldBlue(database), printer.BoldBlue(ch.Config.Organization))
				default:
					return cmdutil.HandleError(err)
				}
			}

			end()

			if len(branches) == 0 && ch.Printer.Format() == printer.Human {
				if flags.page == 0 {
					ch.Printer.Printf("No branches exist in %s.\n", printer.BoldBlue(database))
				} else {
					ch.Printer.Println("No branches found on this page.")
				}
				return nil
			}

			return ch.Printer.PrintResource(toPostgresBranches(branches))
		},
	}

	cmd.Flags().BoolP("web", "w", false, "List branches in your web browser.")
	cmd.Flags().IntVar(&flags.page, "page", 0, "Page number to fetch")
	cmd.Flags().IntVar(&flags.perPage, "per-page", 100, "Number of results per page")
	return cmd
}
