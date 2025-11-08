package cmd

import (
	"context"
	"fmt"
	"time"

	"kc/internal/keycloak"

	"github.com/spf13/cobra"
)

var realmsCmd = &cobra.Command{
	Use:   "realms",
	Short: "Manage realms",
}

var realmsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List realms",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}
		realms, err := client.GetRealms(ctx, token)
		if err != nil {
			return err
		}
		for _, r := range realms {
			if r.Realm != nil {
				fmt.Fprintln(cmd.OutOrStdout(), *r.Realm)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Total: %d\n", len(realms))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(realmsCmd)
	realmsCmd.AddCommand(realmsListCmd)
}
