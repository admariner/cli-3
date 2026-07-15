package role

import (
	"fmt"

	"github.com/planetscale/cli/internal/cmdutil"
	ps "github.com/planetscale/cli/internal/planetscale"
	"github.com/planetscale/cli/internal/printer"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

// ListCmd encapsulates the command for listing roles for a branch.
func ListCmd(ch *cmdutil.Helper) *cobra.Command {
	var flags struct {
		page    int
		perPage int
	}

	cmd := &cobra.Command{
		Use:     "list <database> <branch>",
		Short:   "List all roles for a Postgres database branch",
		Args:    cmdutil.RequiredArgs("database", "branch"),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			database := args[0]
			branch := args[1]

			web, err := cmd.Flags().GetBool("web")
			if err != nil {
				return err
			}

			if web {
				fmt.Println("🌐  Redirecting you to your roles in your web browser.")
				err := browser.OpenURL(fmt.Sprintf("%s/%s/%s/settings/roles", cmdutil.ApplicationURL, ch.Config.Organization, database))
				if err != nil {
					return err
				}
				return nil
			}

			client, err := ch.Client()
			if err != nil {
				return err
			}

			forMsg := fmt.Sprintf("%s/%s", printer.BoldBlue(database), printer.BoldBlue(branch))

			end := ch.Printer.PrintProgress(fmt.Sprintf("Fetching roles for %s", forMsg))
			defer end()

			roles, err := client.PostgresRoles.List(ctx, &ps.ListPostgresRolesRequest{
				Organization: ch.Config.Organization,
				Database:     database,
				Branch:       branch,
			}, ps.WithPage(flags.page), ps.WithPerPage(flags.perPage))
			if err != nil {
				switch cmdutil.ErrCode(err) {
				case ps.ErrNotFound:
					return cmdutil.HandleNotFoundWithServiceTokenCheck(
						ctx, cmd, ch.Config, ch.Client, err,
						"read_branch",
						"database %s or branch %s does not exist in organization %s",
						printer.BoldBlue(database), printer.BoldBlue(branch), printer.BoldBlue(ch.Config.Organization))
				default:
					return cmdutil.HandleError(err)
				}
			}

			end()

			if len(roles) == 0 && ch.Printer.Format() == printer.Human {
				if flags.page == 0 {
					ch.Printer.Printf("No roles exist in %s.\n", forMsg)
				} else {
					ch.Printer.Println("No roles found on this page.")
				}
				return nil
			}

			return ch.Printer.PrintResource(toPostgresRoles(roles))
		},
	}

	cmd.Flags().BoolP("web", "w", false, "List roles in your web browser.")
	cmd.Flags().IntVar(&flags.page, "page", 0, "Page number to fetch")
	cmd.Flags().IntVar(&flags.perPage, "per-page", 100, "Number of results per page")
	return cmd
}

type PostgresRoleList struct {
	PublicID        string `header:"id" json:"id"`
	Name            string `header:"name" json:"name"`
	Username        string `header:"username" json:"username"`
	AccessHostURL   string `header:"access_host_url" json:"access_host_url"`
	WithReplication bool   `header:"with_replication" json:"with_replication"`
	CreatedAt       string `header:"created_at" json:"created_at"`

	orig *ps.PostgresRole
}

func toPostgresRoles(roles []*ps.PostgresRole) []*PostgresRoleList {
	psRoles := make([]*PostgresRoleList, 0, len(roles))

	for _, role := range roles {
		psRoles = append(psRoles, &PostgresRoleList{
			PublicID:        role.ID,
			Name:            role.Name,
			Username:        role.Username,
			AccessHostURL:   role.AccessHostURL,
			WithReplication: role.WithReplication,
			CreatedAt:       role.CreatedAt.Format("2006-01-02 15:04:05"),
			orig:            role,
		})
	}

	return psRoles
}
