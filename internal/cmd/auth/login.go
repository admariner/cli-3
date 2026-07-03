package auth

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"

	"github.com/hashicorp/go-cleanhttp"
	psauth "github.com/planetscale/cli/internal/auth"
	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/printer"
	"github.com/planetscale/planetscale-go/planetscale"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// LoginCmd is the command for logging into a PlanetScale account.
func LoginCmd(ch *cmdutil.Helper) *cobra.Command {
	var clientID string
	var clientSecret string
	var authURL string

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.ExactArgs(0),
		Short: "Authenticate with the PlanetScale API",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode := ch.Printer.Format() == printer.JSON
			if !jsonMode && !printer.IsTTY {
				return errors.New("the 'login' command requires an interactive shell (use --format json; browser opens when possible, then polls until approved)")
			}

			authenticator, err := psauth.New(cleanhttp.DefaultClient(), clientID, clientSecret, psauth.SetBaseURL(authURL))
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			deviceVerification, err := authenticator.VerifyDevice(ctx)
			if err != nil {
				return err
			}

			browserOpened := cmdutil.TryOpenBrowser(runtime.GOOS, deviceVerification.VerificationCompleteURL) == nil

			if jsonMode {
				message := loginPendingMessage(browserOpened)
				pending := LoginPendingResponse{
					Status:          "pending",
					VerificationURL: deviceVerification.VerificationCompleteURL,
					UserCode:        deviceVerification.UserCode,
					BrowserOpened:   browserOpened,
					Message:         message,
					NextSteps: []string{
						"Approve access in the browser",
						cmdutil.AgentAuthCheckCmd(),
					},
				}
				if err := ch.Printer.PrintJSON(pending); err != nil {
					return err
				}
			} else {
				if !browserOpened {
					ch.Printer.Printf("Failed to open a browser; open this URL manually: %s\n", printer.Bold(deviceVerification.VerificationCompleteURL))
				}

				bold := color.New(color.Bold)
				bold.Printf("\nConfirmation Code: ")
				boldGreen := bold.Add(color.FgGreen)
				boldGreen.Fprintln(color.Output, deviceVerification.UserCode)

				ch.Printer.Printf("\nIf something goes wrong, copy and paste this URL into your browser: %s\n\n", printer.Bold(deviceVerification.VerificationCompleteURL))
			}

			var end func()
			if jsonMode {
				fmt.Fprintln(cmd.ErrOrStderr(), "Waiting for browser authorization...")
			} else {
				end = ch.Printer.PrintProgress("Waiting for confirmation...")
				defer end()
			}

			accessToken, err := authenticator.GetAccessTokenForDevice(ctx, *deviceVerification)
			if err != nil {
				return err
			}

			err = config.WriteAccessToken(accessToken)
			if err != nil {
				configDir, configErr := config.ConfigDir()
				if configErr != nil {
					ch.Printer.Printf("Error looking up configuration directory: %s\n", printer.BoldRed(configErr.Error()))
				}
				return fmt.Errorf("error logging in: %w\n\nPlease ensure you have write permissions to the configuration directory: %s", err, configDir)
			}

			if end != nil {
				end()
			}

			if err := writeDefaultOrganizationIfNeeded(ctx, ch, accessToken, authURL); err != nil {
				return err
			}

			if jsonMode {
				ok := LoginOKResponse{
					Status:  "ok",
					Message: "Successfully logged in.",
					NextSteps: []string{
						cmdutil.AgentAuthCheckCmd(),
						cmdutil.AgentOrgListCmd(),
					},
				}
				return ch.Printer.PrintJSON(ok)
			}

			ch.Printer.Println("Successfully logged in.")
			return nil
		},
	}

	cmd.Flags().StringVar(&clientID, "client-id", psauth.OAuthClientID, "The client ID for the PlanetScale CLI application.")
	cmd.Flags().StringVar(&clientSecret, "client-secret", psauth.OAuthClientSecret, "The client ID for the PlanetScale CLI application")
	cmd.Flags().StringVar(&authURL, "api-url", psauth.DefaultBaseURL, "The PlanetScale Auth API base URL.")

	return cmd
}

func writeDefaultOrganizationIfNeeded(ctx context.Context, ch *cmdutil.Helper, accessToken, authURL string) error {
	writeConfig := false
	cfg, err := ch.ConfigFS.DefaultConfig()
	if err != nil {
		writeConfig = true
	}

	if !writeConfig && cfg.Organization != "" {
		hasOrg, err := hasOrg(ctx, cfg.Organization, accessToken, authURL)
		if err != nil {
			return err
		}
		writeConfig = !hasOrg
	}

	if writeConfig || cfg.Organization == "" {
		return writeDefaultOrganization(ctx, accessToken, authURL)
	}
	return nil
}

func writeDefaultOrganization(ctx context.Context, accessToken, authURL string) error {
	orgs, err := listCurrentOrgs(ctx, accessToken, authURL)
	if err != nil {
		return err
	}

	if len(orgs) > 0 {
		defaultOrg := orgs[0].Name
		writableConfig := &config.FileConfig{
			Organization: defaultOrg,
		}

		err := writableConfig.WriteDefault()
		if err != nil {
			return err
		}
	}

	return nil
}

func hasOrg(ctx context.Context, org, accessToken, authURL string) (bool, error) {
	currentOrgs, err := listCurrentOrgs(ctx, accessToken, authURL)
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(currentOrgs, func(o *planetscale.Organization) bool {
		return o.Name == org
	}), nil
}

func listCurrentOrgs(ctx context.Context, accessToken, authURL string) ([]*planetscale.Organization, error) {
	client, err := planetscale.NewClient(
		planetscale.WithAccessToken(accessToken),
		planetscale.WithBaseURL(authURL),
	)
	if err != nil {
		return nil, cmdutil.HandleError(err)
	}

	orgs, err := client.Organizations.List(ctx)
	if err != nil {
		return nil, cmdutil.HandleError(err)
	}

	return orgs, nil
}
