package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"kc/internal/config"
	"kc/internal/keycloak"

	"github.com/Nerzal/gocloak/v13"
	"github.com/spf13/cobra"
)

var (
	csNames        []string
	csDescriptions []string
	csProtocols    []string
	csNewNames     []string
	csAllRealms    bool
	csRealm        string
	csIgnoreMiss   bool
)

var clientScopesCmd = &cobra.Command{
	Use:   "client-scopes",
	Short: "Manage client scopes",
}

func resolveCSRealms() ([]string, error) {
	if csAllRealms {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil { return nil, err }
		rs, err := gc.GetRealms(ctx, token)
		if err != nil { return nil, err }
		var out []string
		for _, r := range rs { if r.Realm != nil { out = append(out, *r.Realm) } }
		return out, nil
	}
	r := csRealm
	if r == "" { r = defaultRealm }
	if r == "" { r = config.Global.Realm }
	if r == "" { return nil, errors.New("target realm not specified. Use --realm or set realm in config.json") }
	return []string{r}, nil
}

func findClientScopeByName(ctx context.Context, gc *gocloak.GoCloak, token, realm, name string) (*gocloak.ClientScope, error) {
	scopes, err := gc.GetClientScopes(ctx, token, realm)
	if err != nil { return nil, err }
	for _, s := range scopes {
		if s.Name != nil && *s.Name == name { return s, nil }
	}
	return nil, fmt.Errorf("client scope %q not found", name)
}

var clientScopesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create client scope(s)",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(csNames) == 0 { return errors.New("missing --name: provide at least one --name") }
		if !(len(csDescriptions) == 0 || len(csDescriptions) == 1 || len(csDescriptions) == len(csNames)) {
			return fmt.Errorf("invalid descriptions: pass none, one (applies to all), or one per --name")
		}
		if !(len(csProtocols) == 0 || len(csProtocols) == 1 || len(csProtocols) == len(csNames)) {
			return fmt.Errorf("invalid protocols: pass none, one (applies to all), or one per --name")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil { return err }
		realms, err := resolveCSRealms()
		if err != nil { return err }
		created, skipped := 0, 0
		for _, realm := range realms {
			for i, n := range csNames {
				// exists?
				if _, err := findClientScopeByName(ctx, gc, token, realm, n); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Client scope %q already exists in realm %q. Skipped.\n", n, realm)
					skipped++
					continue
				}
				desc := ""
				if len(csDescriptions) == 1 { desc = csDescriptions[0] } else if len(csDescriptions) == len(csNames) { desc = csDescriptions[i] }
				protocol := ""
				if len(csProtocols) == 1 { protocol = csProtocols[0] } else if len(csProtocols) == len(csNames) { protocol = csProtocols[i] } else { protocol = "openid-connect" }
				s := gocloak.ClientScope{Name: &n, Description: &desc, Protocol: &protocol}
				id, err := gc.CreateClientScope(ctx, token, realm, s)
				if err != nil {
					if strings.Contains(strings.ToLower(err.Error()), "409") {
						fmt.Fprintf(cmd.OutOrStdout(), "Client scope %q already exists in realm %q. Skipped.\n", n, realm)
						skipped++
						continue
					}
					return fmt.Errorf("failed creating client scope %q in realm %s: %w", n, realm, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created client scope %q (ID: %s) in realm %q.\n", n, id, realm)
				created++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Done. Created: %d, Skipped: %d.\n", created, skipped)
		return nil
	}),
}

var clientScopesUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update client scope(s)",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(csNames) == 0 { return errors.New("missing --name: provide at least one --name") }
		if len(csDescriptions) == 0 && len(csProtocols) == 0 && len(csNewNames) == 0 { return errors.New("nothing to update: provide --description/--protocol/--new-name") }
		if !(len(csDescriptions) == 0 || len(csDescriptions) == 1 || len(csDescriptions) == len(csNames)) { return fmt.Errorf("invalid descriptions") }
		if !(len(csProtocols) == 0 || len(csProtocols) == 1 || len(csProtocols) == len(csNames)) { return fmt.Errorf("invalid protocols") }
		if !(len(csNewNames) == 0 || len(csNewNames) == 1 || len(csNewNames) == len(csNames)) { return fmt.Errorf("invalid new-name list") }
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil { return err }
		realms, err := resolveCSRealms()
		if err != nil { return err }
		updated, skipped := 0, 0
		for _, realm := range realms {
			for i, n := range csNames {
				scope, err := findClientScopeByName(ctx, gc, token, realm, n)
				if err != nil {
					if csIgnoreMiss { fmt.Fprintf(cmd.OutOrStdout(), "Client scope %q not found in realm %q. Skipped.\n", n, realm); skipped++; continue }
					return fmt.Errorf("client scope %q not found in realm %s", n, realm)
				}
				if len(csDescriptions) == 1 { scope.Description = &csDescriptions[0] } else if len(csDescriptions) == len(csNames) { scope.Description = &csDescriptions[i] }
				if len(csProtocols) == 1 { scope.Protocol = &csProtocols[0] } else if len(csProtocols) == len(csNames) { scope.Protocol = &csProtocols[i] }
				if len(csNewNames) == 1 { scope.Name = &csNewNames[0] } else if len(csNewNames) == len(csNames) { scope.Name = &csNewNames[i] }
				if err := gc.UpdateClientScope(ctx, token, realm, *scope); err != nil {
					return fmt.Errorf("failed updating client scope %q in realm %s: %w", n, realm, err)
				}
				finalName := n
				if scope.Name != nil { finalName = *scope.Name }
				fmt.Fprintf(cmd.OutOrStdout(), "Updated client scope %q in realm %q. New name: %q.\n", n, realm, finalName)
				updated++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Done. Updated: %d, Skipped: %d.\n", updated, skipped)
		return nil
	}),
}

var clientScopesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete client scope(s)",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(csNames) == 0 { return errors.New("missing --name: provide at least one --name") }
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil { return err }
		realms, err := resolveCSRealms()
		if err != nil { return err }
		deleted, skipped := 0, 0
		for _, realm := range realms {
			for _, n := range csNames {
				scope, err := findClientScopeByName(ctx, gc, token, realm, n)
				if err != nil {
					if csIgnoreMiss { fmt.Fprintf(cmd.OutOrStdout(), "Client scope %q not found in realm %q. Skipped.\n", n, realm); skipped++; continue }
					return fmt.Errorf("client scope %q not found in realm %s", n, realm)
				}
				if err := gc.DeleteClientScope(ctx, token, realm, *scope.ID); err != nil {
					return fmt.Errorf("failed deleting client scope %q in realm %s: %w", n, realm, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted client scope %q (ID: %s) in realm %q.\n", n, *scope.ID, realm)
				deleted++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Done. Deleted: %d, Skipped: %d.\n", deleted, skipped)
		return nil
	}),
}

var clientScopesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List client scopes",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil { return err }
		realms, err := resolveCSRealms()
		if err != nil { return err }
		total := 0
		for _, realm := range realms {
			scopes, err := gc.GetClientScopes(ctx, token, realm)
			if err != nil { return err }
			for _, s := range scopes {
				if s.Name != nil { fmt.Fprintln(cmd.OutOrStdout(), *s.Name); total++ }
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Total: %d\n", total)
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(clientScopesCmd)
	clientScopesCmd.AddCommand(clientScopesCreateCmd)
	clientScopesCreateCmd.Flags().StringSliceVar(&csNames, "name", nil, "client scope name(s). Repeatable; required.")
	clientScopesCreateCmd.Flags().StringSliceVar(&csDescriptions, "description", nil, "description(s). Optional; 0,1 or N")
	clientScopesCreateCmd.Flags().StringSliceVar(&csProtocols, "protocol", nil, "protocol(s). Optional; 0,1 or N; default openid-connect")
	clientScopesCreateCmd.Flags().BoolVar(&csAllRealms, "all-realms", false, "create in all realms")
	clientScopesCreateCmd.Flags().StringVar(&csRealm, "realm", "", "target realm")

	clientScopesCmd.AddCommand(clientScopesUpdateCmd)
	clientScopesUpdateCmd.Flags().StringSliceVar(&csNames, "name", nil, "client scope name(s) to update. Repeatable; required.")
	clientScopesUpdateCmd.Flags().StringSliceVar(&csDescriptions, "description", nil, "new description(s). Optional; 0,1 or N")
	clientScopesUpdateCmd.Flags().StringSliceVar(&csProtocols, "protocol", nil, "new protocol(s). Optional; 0,1 or N")
	clientScopesUpdateCmd.Flags().StringSliceVar(&csNewNames, "new-name", nil, "new name(s). Optional; 0,1 or N")
	clientScopesUpdateCmd.Flags().BoolVar(&csAllRealms, "all-realms", false, "update in all realms")
	clientScopesUpdateCmd.Flags().StringVar(&csRealm, "realm", "", "target realm")
	clientScopesUpdateCmd.Flags().BoolVar(&csIgnoreMiss, "ignore-missing", false, "skip scopes not found instead of failing")

	clientScopesCmd.AddCommand(clientScopesDeleteCmd)
	clientScopesDeleteCmd.Flags().StringSliceVar(&csNames, "name", nil, "client scope name(s) to delete. Repeatable; required.")
	clientScopesDeleteCmd.Flags().BoolVar(&csAllRealms, "all-realms", false, "delete in all realms")
	clientScopesDeleteCmd.Flags().StringVar(&csRealm, "realm", "", "target realm")
	clientScopesDeleteCmd.Flags().BoolVar(&csIgnoreMiss, "ignore-missing", false, "skip scopes not found instead of failing")

	clientScopesCmd.AddCommand(clientScopesListCmd)
	clientScopesListCmd.Flags().BoolVar(&csAllRealms, "all-realms", false, "list in all realms")
	clientScopesListCmd.Flags().StringVar(&csRealm, "realm", "", "target realm")
}
