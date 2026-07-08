package branch

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
	ps "github.com/planetscale/planetscale-go/planetscale"

	"github.com/spf13/cobra"
)

type InfraPod struct {
	Component     string `header:"component" json:"component"`
	Size          string `header:"size" json:"size"`
	Cell          string `header:"cell" json:"cell"`
	TabletType    string `header:"tablet type" json:"tablet_type"`
	KeyspaceShard string `header:"keyspace/shard" json:"keyspace_shard"`
	Ready         string `header:"ready" json:"ready"`
	Restarts      int    `header:"restarts" json:"restarts"`
	Status        string `header:"status" json:"status"`
	Age           string `header:"age" json:"age"`
	Name          string `header:"pod name" json:"name"`

	orig *ps.BranchInfraPod
}

func (p *InfraPod) MarshalJSON() ([]byte, error) {
	return json.MarshalIndent(p.orig, "", "  ")
}

func (p *InfraPod) MarshalCSVValue() interface{} {
	return []*InfraPod{p}
}

func toInfraPod(pod *ps.BranchInfraPod) *InfraPod {
	tabletType := "-"
	if pod.TabletType != nil {
		tabletType = *pod.TabletType
	}

	keyspaceShard := "-"
	if pod.Keyspace != nil && pod.Shard != nil {
		keyspaceShard = fmt.Sprintf("%s/%s", *pod.Keyspace, *pod.Shard)
	}

	age := "-"
	if pod.CreatedAt != nil {
		age = humanAge(time.Since(*pod.CreatedAt))
	}

	return &InfraPod{
		Component:     pod.Component,
		Size:          pod.Size,
		Cell:          pod.Cell,
		TabletType:    tabletType,
		KeyspaceShard: keyspaceShard,
		Ready:         pod.Ready,
		Restarts:      pod.RestartCount,
		Status:        pod.Status,
		Age:           age,
		Name:          pod.Name,
		orig:          pod,
	}
}

func toInfraPods(pods []*ps.BranchInfraPod) []*InfraPod {
	result := make([]*InfraPod, 0, len(pods))
	for _, pod := range pods {
		result = append(result, toInfraPod(pod))
	}
	return result
}

type PostgresInfraRow struct {
	Component string `header:"component" json:"component"`
	Role      string `header:"role" json:"role"`
	Size      string `header:"size" json:"size"`
	AZ        string `header:"az" json:"az"`
	Storage   string `header:"storage" json:"storage"`
	Name      string `header:"name" json:"name"`

	orig any
}

func (r *PostgresInfraRow) MarshalJSON() ([]byte, error) {
	return json.MarshalIndent(r.orig, "", "  ")
}

func (r *PostgresInfraRow) MarshalCSVValue() interface{} {
	return []*PostgresInfraRow{r}
}

func toPostgresInfraRows(pg *ps.PostgresBranchInfrastructure) []*PostgresInfraRow {
	rows := make([]*PostgresInfraRow, 0, len(pg.Nodes)+len(pg.Bouncers))

	for _, node := range pg.Nodes {
		storage := "-"
		if node.VolumeUsageBytes != nil && node.VolumeCapacityBytes != nil {
			storage = fmt.Sprintf("%s / %s",
				humanize.IBytes(uint64(*node.VolumeUsageBytes)),
				humanize.IBytes(uint64(*node.VolumeCapacityBytes)))
		}

		rows = append(rows, &PostgresInfraRow{
			Component: "postgres",
			Role:      node.Role,
			Size:      node.ClusterDisplayName,
			AZ:        node.AvailabilityZone,
			Storage:   storage,
			Name:      node.Name,
			orig:      node,
		})
	}

	for _, bouncer := range pg.Bouncers {
		size := "-"
		if bouncer.SKU != nil {
			size = bouncer.SKU.DisplayName
		}

		rows = append(rows, &PostgresInfraRow{
			Component: "pgbouncer",
			Role:      bouncer.Target,
			Size:      size,
			AZ:        "-",
			Storage:   "-",
			Name:      bouncer.Name,
			orig:      bouncer,
		})
	}

	return rows
}

func humanAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	if h == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, h)
}

// InfraCmd shows infrastructure (pods) for a branch.
func InfraCmd(ch *cmdutil.Helper) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "infra <database> <branch>",
		Short:  "Show infrastructure (pods) for a branch",
		Hidden: true,
		Args:   cmdutil.RequiredArgs("database", "branch"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			database := args[0]
			branch := args[1]

			client, err := ch.Client()
			if err != nil {
				return err
			}

			end := ch.Printer.PrintProgress(fmt.Sprintf("Fetching infrastructure for %s/%s",
				printer.BoldBlue(database), printer.BoldBlue(branch)))
			defer end()

			infra, err := client.BranchInfrastructure.Get(ctx, &ps.GetBranchInfrastructureRequest{
				Organization: ch.Config.Organization,
				Database:     database,
				Branch:       branch,
			})
			if err != nil {
				switch cmdutil.ErrCode(err) {
				case ps.ErrNotFound:
					return cmdutil.HandleNotFoundWithServiceTokenCheck(
						ctx, cmd, ch.Config, ch.Client, err, "read_branch",
						"branch %s does not exist in database %s (organization: %s)",
						printer.BoldBlue(branch), printer.BoldBlue(database), printer.BoldBlue(ch.Config.Organization))
				default:
					return cmdutil.HandleError(err)
				}
			}

			end()

			switch {
			case infra.Postgres != nil:
				rows := toPostgresInfraRows(infra.Postgres)
				if len(rows) == 0 && ch.Printer.Format() == printer.Human {
					ch.Printer.Printf("No nodes found for branch %s.\n", printer.BoldBlue(branch))
					return nil
				}

				return ch.Printer.PrintResource(rows)
			case infra.Vitess != nil:
				if len(infra.Vitess.Pods) == 0 && ch.Printer.Format() == printer.Human {
					ch.Printer.Printf("No pods found for branch %s.\n", printer.BoldBlue(branch))
					return nil
				}

				return ch.Printer.PrintResource(toInfraPods(infra.Vitess.Pods))
			default:
				return fmt.Errorf("unexpected infrastructure response for branch %s", branch)
			}
		},
	}

	return cmd
}
