package auth

import (
	psauth "github.com/planetscale/cli/internal/auth"

	"github.com/spf13/cobra"
)

// addOAuthClientFlags registers --client-id and --client-secret without exposing
// the built-in OAuth credentials as help-text defaults.
func addOAuthClientFlags(cmd *cobra.Command, clientID, clientSecret *string) {
	cmd.Flags().StringVar(clientID, "client-id", "", "The client ID for the PlanetScale CLI application.")
	cmd.Flags().StringVar(clientSecret, "client-secret", "", "The client secret for the PlanetScale CLI application.")
}

// resolveOAuthClient returns the flag values, falling back to the built-in
// PlanetScale CLI OAuth application credentials when unset.
func resolveOAuthClient(clientID, clientSecret string) (string, string) {
	if clientID == "" {
		clientID = psauth.OAuthClientID
	}
	if clientSecret == "" {
		clientSecret = psauth.OAuthClientSecret
	}
	return clientID, clientSecret
}
