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
	cliIDs             []string
	cliNames           []string
	cliPublics         []bool
	cliSecrets         []string
	cliEnabled         []bool
	cliProtocols       []string
	cliRootURLs        []string
	cliBaseURLs        []string
	cliRedirectURIs    [][]string
	cliWebOrigins      [][]string
	cliStandardFlows   []bool
	cliDirectAccess    []bool
	cliImplicitFlows   []bool
	cliServiceAccounts []bool
	cliNewClientIDs    []string
	clientsRealms      []string
	clientsAllRealms   bool
	clientsIgnoreMiss  bool

	// scopes subcommand
	scopeClientID   string
	scopeNames      []string
	scopeType       string // default | optional
	scopeIgnoreMiss bool
)

var clientsCmd = &cobra.Command{
	Use:   "clients",
	Short: "Manage clients",
}

func resolveRealmsForClients(cmd *cobra.Command) ([]string, error) {
	if clientsAllRealms {
		ctx := cmd.Context()
		client, token, err := keycloak.Login(ctx)
		if err != nil {
			return nil, err
		}
		realms, err := client.GetRealms(ctx, token)
		if err != nil {
			return nil, err
		}
		var rs []string
		for _, r := range realms {
			if r.Realm != nil {
				rs = append(rs, *r.Realm)
			}
		}
		return rs, nil
	}
	if len(clientsRealms) > 0 {
		return append([]string{}, clientsRealms...), nil
	}
	r := defaultRealm
	if r == "" {
		r = config.Global.Realm
	}
	if r == "" {
		return nil, errors.New("target realm not specified. Use --realm or set realm in config.json")
	}
	return []string{r}, nil
}

// Helper to pick value 0/1/N aligned to index i
func pick[T any](vals []T, i int) (T, bool) {
	var zero T
	if len(vals) == 1 {
		return vals[0], true
	}
	if len(vals) > 1 {
		return vals[i], true
	}
	return zero, false
}

func getClientByClientID(ctx context.Context, gc *gocloak.GoCloak, token, realm, cid string) (*gocloak.Client, error) {
	params := gocloak.GetClientsParams{ClientID: &cid}
	list, err := gc.GetClients(ctx, token, realm, params)
	if err != nil {
		return nil, err
	}
	for _, c := range list {
		if c.ClientID != nil && *c.ClientID == cid {
			return c, nil
		}
	}
	return nil, fmt.Errorf("client %q not found", cid)
}

var clientsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create client(s)",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(cliIDs) == 0 {
			return errors.New("missing --client-id: provide at least one --client-id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}

		realms, err := resolveRealmsForClients(cmd)
		if err != nil {
			return err
		}

		created, skipped := 0, 0
		var lines []string
		for _, realm := range realms {
			for i, cid := range cliIDs {
				// existence
				// existence via GetClients filter
				existing, err := getClientByClientID(ctx, gc, token, realm, cid)
				if err == nil && existing != nil && existing.ID != nil {
					lines = append(lines, fmt.Sprintf("Client %q already exists in realm %q. Skipped.", cid, realm))
					skipped++
					continue
				}
				var name, secret, protocol, rootURL, baseURL string
				if v, ok := pick(cliNames, i); ok {
					name = v
				}
				if v, ok := pick(cliSecrets, i); ok {
					secret = v
				}
				if v, ok := pick(cliProtocols, i); ok {
					protocol = v
				}
				if v, ok := pick(cliRootURLs, i); ok {
					rootURL = v
				}
				if v, ok := pick(cliBaseURLs, i); ok {
					baseURL = v
				}
				var enabled, publicClient, stdFlow, direct, implicit, svcAcct bool
				if v, ok := pick(cliEnabled, i); ok {
					enabled = v
				} else {
					enabled = true
				}
				if v, ok := pick(cliPublics, i); ok {
					publicClient = v
				}
				if v, ok := pick(cliStandardFlows, i); ok {
					stdFlow = v
				}
				if v, ok := pick(cliDirectAccess, i); ok {
					direct = v
				}
				if v, ok := pick(cliImplicitFlows, i); ok {
					implicit = v
				}
				if v, ok := pick(cliServiceAccounts, i); ok {
					svcAcct = v
				}

				cl := gocloak.Client{ClientID: &cid}
				if name != "" {
					cl.Name = &name
				}
				cl.Enabled = &enabled
				cl.PublicClient = &publicClient
				if protocol != "" {
					cl.Protocol = &protocol
				}
				if rootURL != "" {
					cl.RootURL = &rootURL
				}
				if baseURL != "" {
					cl.BaseURL = &baseURL
				}
				if stdFlow {
					cl.StandardFlowEnabled = &stdFlow
				}
				if direct {
					cl.DirectAccessGrantsEnabled = &direct
				}
				if implicit {
					cl.ImplicitFlowEnabled = &implicit
				}
				if svcAcct {
					cl.ServiceAccountsEnabled = &svcAcct
				}

				id, err := gc.CreateClient(ctx, token, realm, cl)
				if err != nil {
					// if 409 already exists (rare), treat as skipped
					if strings.Contains(strings.ToLower(err.Error()), "409") {
						fmt.Fprintf(cmd.OutOrStdout(), "Client %q already exists in realm %q. Skipped.\n", cid, realm)
						skipped++
						continue
					}
					return fmt.Errorf("failed creating client %q in realm %s: %w", cid, realm, err)
				}

				// explicit secret setting is not supported by gocloak (only regenerate). If provided, warn and continue.
				if secret != "" && !publicClient {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: --secret provided for client %q but explicit secret setting is not supported. Skipped setting secret.\n", cid)
				}

				// Redirect URIs and Web Origins
				if i < len(cliRedirectURIs) && len(cliRedirectURIs[i]) > 0 {
					if err := gc.UpdateClient(ctx, token, realm, gocloak.Client{ID: &id, RedirectURIs: &cliRedirectURIs[i]}); err != nil {
						return fmt.Errorf("failed setting redirect URIs for client %q in realm %s: %w", cid, realm, err)
					}
				}
				if i < len(cliWebOrigins) && len(cliWebOrigins[i]) > 0 {
					if err := gc.UpdateClient(ctx, token, realm, gocloak.Client{ID: &id, WebOrigins: &cliWebOrigins[i]}); err != nil {
						return fmt.Errorf("failed setting web origins for client %q in realm %s: %w", cid, realm, err)
					}
				}

				lines = append(lines, fmt.Sprintf("Created client %q (ID: %s) in realm %q.", cid, id, realm))
				created++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Created: %d, Skipped: %d.", created, skipped))
		realmLabel := ""
		if clientsAllRealms {
			realmLabel = "all realms"
		} else if len(clientsRealms) == 1 {
			realmLabel = clientsRealms[0]
		} else if len(realms) == 1 {
			realmLabel = realms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

var clientsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update client(s)",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(cliIDs) == 0 {
			return errors.New("missing --client-id: provide at least one --client-id")
		}
		// Must have at least one field to update
		any := len(cliNames) > 0 || len(cliPublics) > 0 || len(cliSecrets) > 0 || len(cliEnabled) > 0 || len(cliProtocols) > 0 || len(cliRootURLs) > 0 || len(cliBaseURLs) > 0 || len(cliRedirectURIs) > 0 || len(cliWebOrigins) > 0 || len(cliStandardFlows) > 0 || len(cliDirectAccess) > 0 || len(cliImplicitFlows) > 0 || len(cliServiceAccounts) > 0 || len(cliNewClientIDs) > 0
		if !any {
			return errors.New("nothing to update: provide at least one field flag")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}
		realms, err := resolveRealmsForClients(cmd)
		if err != nil {
			return err
		}

		updated, skipped := 0, 0
		var lines []string
		for _, realm := range realms {
			for i, cid := range cliIDs {
				c, err := getClientByClientID(ctx, gc, token, realm, cid)
				if err != nil || c == nil || c.ID == nil {
					if clientsIgnoreMiss {
						lines = append(lines, fmt.Sprintf("Client %q not found in realm %q. Skipped.", cid, realm))
						skipped++
						continue
					}
					return fmt.Errorf("client %q not found in realm %s", cid, realm)
				}
				id := *c.ID
				// Apply updates
				if v, ok := pick(cliNames, i); ok {
					c.Name = &v
				}
				if v, ok := pick(cliPublics, i); ok {
					c.PublicClient = &v
				}
				if v, ok := pick(cliEnabled, i); ok {
					c.Enabled = &v
				}
				if v, ok := pick(cliProtocols, i); ok {
					c.Protocol = &v
				}
				if v, ok := pick(cliRootURLs, i); ok {
					c.RootURL = &v
				}
				if v, ok := pick(cliBaseURLs, i); ok {
					c.BaseURL = &v
				}
				if v, ok := pick(cliStandardFlows, i); ok {
					c.StandardFlowEnabled = &v
				}
				if v, ok := pick(cliDirectAccess, i); ok {
					c.DirectAccessGrantsEnabled = &v
				}
				if v, ok := pick(cliImplicitFlows, i); ok {
					c.ImplicitFlowEnabled = &v
				}
				if v, ok := pick(cliServiceAccounts, i); ok {
					c.ServiceAccountsEnabled = &v
				}
				if i < len(cliRedirectURIs) && len(cliRedirectURIs[i]) > 0 {
					c.RedirectURIs = &cliRedirectURIs[i]
				}
				if i < len(cliWebOrigins) && len(cliWebOrigins[i]) > 0 {
					c.WebOrigins = &cliWebOrigins[i]
				}

				if err := gc.UpdateClient(ctx, token, realm, *c); err != nil {
					return fmt.Errorf("failed updating client %q in realm %s: %w", cid, realm, err)
				}
				if v, ok := pick(cliSecrets, i); ok && v != "" && (c.PublicClient == nil || !*c.PublicClient) {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: --secret provided for client %q but explicit secret setting is not supported. Skipped setting secret.\n", cid)
				}
				if v, ok := pick(cliNewClientIDs, i); ok && v != "" {
					c.ClientID = &v
					if err := gc.UpdateClient(ctx, token, realm, *c); err != nil {
						return fmt.Errorf("failed renaming client %q to %q in realm %s: %w", cid, v, realm, err)
					}
				}
				lines = append(lines, fmt.Sprintf("Updated client %q (ID: %s) in realm %q.", cid, id, realm))
				updated++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Updated: %d, Skipped: %d.", updated, skipped))
		realmLabel := ""
		if clientsAllRealms {
			realmLabel = "all realms"
		} else if len(clientsRealms) == 1 {
			realmLabel = clientsRealms[0]
		} else if len(realms) == 1 {
			realmLabel = realms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

var clientsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete client(s)",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(cliIDs) == 0 {
			return errors.New("missing --client-id: provide at least one --client-id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}
		realms, err := resolveRealmsForClients(cmd)
		if err != nil {
			return err
		}

		deleted, skipped := 0, 0
		var lines []string
		for _, realm := range realms {
			for _, cid := range cliIDs {
				c, err := getClientByClientID(ctx, gc, token, realm, cid)
				if err != nil || c == nil || c.ID == nil {
					if clientsIgnoreMiss {
						fmt.Fprintf(cmd.OutOrStdout(), "Client %q not found in realm %q. Skipped.\n", cid, realm)
						skipped++
						continue
					}
					return fmt.Errorf("client %q not found in realm %s", cid, realm)
				}
				if err := gc.DeleteClient(ctx, token, realm, *c.ID); err != nil {
					return fmt.Errorf("failed deleting client %q in realm %s: %w", cid, realm, err)
				}
				lines = append(lines, fmt.Sprintf("Deleted client %q (ID: %s) in realm %q.", cid, *c.ID, realm))
				deleted++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Deleted: %d, Skipped: %d.", deleted, skipped))
		realmLabel := ""
		if clientsAllRealms {
			realmLabel = "all realms"
		} else if len(clientsRealms) == 1 {
			realmLabel = clientsRealms[0]
		} else if len(realms) == 1 {
			realmLabel = realms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

var clientsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List clients",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}
		realms, err := resolveRealmsForClients(cmd)
		if err != nil {
			return err
		}

		total := 0
		lines := []string{}
		for _, realm := range realms {
			params := gocloak.GetClientsParams{}
			// when filter by client-id provided as single value, we can use Search or ClientID
			if len(cliIDs) == 1 {
				params.ClientID = &cliIDs[0]
			}
			clients, err := gc.GetClients(ctx, token, realm, params)
			if err != nil {
				return err
			}
			for _, c := range clients {
				if c.ClientID != nil {
					lines = append(lines, *c.ClientID)
					total++
				}
			}
		}
		lines = append(lines, fmt.Sprintf("Total: %d", total))
		realmLabel := ""
		if clientsAllRealms {
			realmLabel = "all realms"
		} else if len(clientsRealms) == 1 {
			realmLabel = clientsRealms[0]
		} else if len(realms) == 1 {
			realmLabel = realms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

var clientsScopesCmd = &cobra.Command{
	Use:   "scopes",
	Short: "Manage client scope assignments",
}

var clientsScopesAssignCmd = &cobra.Command{
	Use:   "assign",
	Short: "Assign client scopes to a client",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if scopeClientID == "" {
			return errors.New("missing --client-id")
		}
		if len(scopeNames) == 0 {
			return errors.New("missing --scope: provide at least one --scope")
		}
		if scopeType != "default" && scopeType != "optional" {
			return errors.New("invalid --type: must be 'default' or 'optional'")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}
		realms, err := resolveRealmsForClients(cmd)
		if err != nil {
			return err
		}

		assigned, skipped := 0, 0
		var lines []string
		for _, realm := range realms {
			client, err := getClientByClientID(ctx, gc, token, realm, scopeClientID)
			if err != nil || client == nil || client.ID == nil {
				return fmt.Errorf("client %q not found in realm %s", scopeClientID, realm)
			}
			clientID := *client.ID
			// cache scopes in realm
			realmScopes, err := gc.GetClientScopes(ctx, token, realm)
			if err != nil {
				return err
			}
			for _, sn := range scopeNames {
				var scopeID string
				for _, sc := range realmScopes {
					if sc.Name != nil && *sc.Name == sn && sc.ID != nil {
						scopeID = *sc.ID
						break
					}
				}
				if scopeID == "" {
					return fmt.Errorf("client scope %q not found in realm %s", sn, realm)
				}
				if scopeType == "default" {
					if err := gc.AddDefaultScopeToClient(ctx, token, realm, clientID, scopeID); err != nil {
						if strings.Contains(strings.ToLower(err.Error()), "409") {
							lines = append(lines, fmt.Sprintf("Scope %q already default for client %q in realm %q. Skipped.", sn, scopeClientID, realm))
							skipped++
							continue
						}
						return fmt.Errorf("failed assigning default scope %q to client %q in realm %s: %w", sn, scopeClientID, realm, err)
					}
				} else {
					if err := gc.AddOptionalScopeToClient(ctx, token, realm, clientID, scopeID); err != nil {
						if strings.Contains(strings.ToLower(err.Error()), "409") {
							lines = append(lines, fmt.Sprintf("Scope %q already optional for client %q in realm %q. Skipped.", sn, scopeClientID, realm))
							skipped++
							continue
						}
						return fmt.Errorf("failed assigning optional scope %q to client %q in realm %s: %w", sn, scopeClientID, realm, err)
					}
				}
				lines = append(lines, fmt.Sprintf("Assigned %s scope %q to client %q in realm %q.", scopeType, sn, scopeClientID, realm))
				assigned++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Assigned: %d, Skipped: %d.", assigned, skipped))
		realmLabel := ""
		if clientsAllRealms {
			realmLabel = "all realms"
		} else if len(clientsRealms) == 1 {
			realmLabel = clientsRealms[0]
		} else if len(realms) == 1 {
			realmLabel = realms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

var clientsScopesRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove client scopes from a client",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if scopeClientID == "" {
			return errors.New("missing --client-id")
		}
		if len(scopeNames) == 0 {
			return errors.New("missing --scope: provide at least one --scope")
		}
		if scopeType != "default" && scopeType != "optional" {
			return errors.New("invalid --type: must be 'default' or 'optional'")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}
		realms, err := resolveRealmsForClients(cmd)
		if err != nil {
			return err
		}

		removed, skipped := 0, 0
		var lines []string
		for _, realm := range realms {
			client, err := getClientByClientID(ctx, gc, token, realm, scopeClientID)
			if err != nil || client == nil || client.ID == nil {
				return fmt.Errorf("client %q not found in realm %s", scopeClientID, realm)
			}
			clientID := *client.ID
			// cache realm scopes
			realmScopes, err := gc.GetClientScopes(ctx, token, realm)
			if err != nil {
				return err
			}
			for _, sn := range scopeNames {
				var scopeID string
				for _, sc := range realmScopes {
					if sc.Name != nil && *sc.Name == sn && sc.ID != nil {
						scopeID = *sc.ID
						break
					}
				}
				if scopeID == "" {
					if scopeIgnoreMiss {
						lines = append(lines, fmt.Sprintf("Client scope %q not found in realm %q. Skipped.", sn, realm))
						skipped++
						continue
					}
					return fmt.Errorf("client scope %q not found in realm %s", sn, realm)
				}
				if scopeType == "default" {
					if err := gc.RemoveDefaultScopeFromClient(ctx, token, realm, clientID, scopeID); err != nil {
						if strings.Contains(strings.ToLower(err.Error()), "404") && scopeIgnoreMiss {
							lines = append(lines, fmt.Sprintf("Default scope %q not assigned to client %q in realm %q. Skipped.", sn, scopeClientID, realm))
							skipped++
							continue
						}
						return fmt.Errorf("failed removing default scope %q from client %q in realm %s: %w", sn, scopeClientID, realm, err)
					}
				} else {
					if err := gc.RemoveOptionalScopeFromClient(ctx, token, realm, clientID, scopeID); err != nil {
						if strings.Contains(strings.ToLower(err.Error()), "404") && scopeIgnoreMiss {
							lines = append(lines, fmt.Sprintf("Optional scope %q not assigned to client %q in realm %q. Skipped.", sn, scopeClientID, realm))
							skipped++
							continue
						}
						return fmt.Errorf("failed removing optional scope %q from client %q in realm %s: %w", sn, scopeClientID, realm, err)
					}
				}
				lines = append(lines, fmt.Sprintf("Removed %s scope %q from client %q in realm %q.", scopeType, sn, scopeClientID, realm))
				removed++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Removed: %d, Skipped: %d.", removed, skipped))
		realmLabel := ""
		if clientsAllRealms {
			realmLabel = "all realms"
		} else if len(clientsRealms) == 1 {
			realmLabel = clientsRealms[0]
		} else if len(realms) == 1 {
			realmLabel = realms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(clientsCmd)

	clientsCmd.AddCommand(clientsCreateCmd)
	clientsCreateCmd.Flags().StringSliceVar(&cliIDs, "client-id", nil, "client-id(s). Repeatable; required.")
	clientsCreateCmd.Flags().StringSliceVar(&cliNames, "name", nil, "name(s). Optional; 0, 1 or N matching --client-id.")
	clientsCreateCmd.Flags().BoolSliceVar(&cliPublics, "public", nil, "public client(s). Optional; 0, 1 or N; default false")
	clientsCreateCmd.Flags().StringSliceVar(&cliSecrets, "secret", nil, "secret(s). Optional; ignored for public clients")
	clientsCreateCmd.Flags().BoolSliceVar(&cliEnabled, "enabled", nil, "enabled flag(s). Optional; 0, 1 or N; default true")
	clientsCreateCmd.Flags().StringSliceVar(&cliProtocols, "protocol", nil, "protocol(s). Optional; 0, 1 or N; e.g. openid-connect")
	clientsCreateCmd.Flags().StringSliceVar(&cliRootURLs, "root-url", nil, "root URL(s). Optional; 0, 1 or N")
	clientsCreateCmd.Flags().StringSliceVar(&cliBaseURLs, "base-url", nil, "base URL(s). Optional; 0, 1 or N")
	// For lists, accept comma-separated via repeated flag usage (cobra handles)
	clientsCreateCmd.Flags().StringSlice("redirect-uri", nil, "redirect URI list per client; repeat flag per client")
	clientsCreateCmd.Flags().StringSlice("web-origin", nil, "web origin list per client; repeat flag per client")
	// Bind the above slice-of-slices manually in PreRunE? We'll parse at runtime: cobra can't directly bind [][]string easily.
	// Approach: users can pass multiple --redirect-uri flags; cobra accumulates into one slice, which can't map per-client cleanly.
	// To keep parity with current style, we'll allow only one list applied to all clients; advanced per-index lists can be added later.
	// Therefore, we override: read once into tmp and apply to all by expanding.

	clientsCmd.AddCommand(clientsUpdateCmd)
	clientsUpdateCmd.Flags().StringSliceVar(&cliIDs, "client-id", nil, "client-id(s) to update. Repeatable; required.")
	clientsUpdateCmd.Flags().StringSliceVar(&cliNames, "name", nil, "new name(s). Optional; 0, 1 or N")
	clientsUpdateCmd.Flags().BoolSliceVar(&cliPublics, "public", nil, "set public flag(s). Optional; 0, 1 or N")
	clientsUpdateCmd.Flags().StringSliceVar(&cliSecrets, "secret", nil, "new secret(s). Optional; ignored for public clients")
	clientsUpdateCmd.Flags().BoolSliceVar(&cliEnabled, "enabled", nil, "set enabled flag(s). Optional; 0, 1 or N")
	clientsUpdateCmd.Flags().StringSliceVar(&cliProtocols, "protocol", nil, "protocol(s). Optional; 0, 1 or N")
	clientsUpdateCmd.Flags().StringSliceVar(&cliRootURLs, "root-url", nil, "root URL(s). Optional; 0, 1 or N")
	clientsUpdateCmd.Flags().StringSliceVar(&cliBaseURLs, "base-url", nil, "base URL(s). Optional; 0, 1 or N")
	clientsUpdateCmd.Flags().StringSlice("redirect-uri", nil, "redirect URI list to replace; applies to all targeted clients")
	clientsUpdateCmd.Flags().StringSlice("web-origin", nil, "web origin list to replace; applies to all targeted clients")
	clientsUpdateCmd.Flags().BoolSliceVar(&cliStandardFlows, "standard-flow", nil, "enable standard flow(s). Optional; 0,1 or N")
	clientsUpdateCmd.Flags().BoolSliceVar(&cliDirectAccess, "direct-access", nil, "enable direct access grants(s). Optional; 0,1 or N")
	clientsUpdateCmd.Flags().BoolSliceVar(&cliImplicitFlows, "implicit-flow", nil, "enable implicit flow(s). Optional; 0,1 or N")
	clientsUpdateCmd.Flags().BoolSliceVar(&cliServiceAccounts, "service-accounts", nil, "enable service accounts(s). Optional; 0,1 or N")
	clientsUpdateCmd.Flags().StringSliceVar(&cliNewClientIDs, "new-client-id", nil, "new client-id(s). Optional; 0,1 or N")
	clientsUpdateCmd.Flags().BoolVar(&clientsIgnoreMiss, "ignore-missing", false, "skip clients not found instead of failing")

	clientsCmd.AddCommand(clientsDeleteCmd)
	clientsDeleteCmd.Flags().StringSliceVar(&cliIDs, "client-id", nil, "client-id(s) to delete. Repeatable; required.")
	clientsDeleteCmd.Flags().BoolVar(&clientsIgnoreMiss, "ignore-missing", false, "skip clients not found instead of failing")

	clientsCmd.AddCommand(clientsListCmd)
	clientsListCmd.Flags().StringSliceVar(&cliIDs, "client-id", nil, "filter by client-id (single value supported)")

	clientsCmd.AddCommand(clientsScopesCmd)
	clientsScopesCmd.AddCommand(clientsScopesAssignCmd)
	clientsScopesCmd.AddCommand(clientsScopesRemoveCmd)
	clientsScopesAssignCmd.Flags().StringVar(&scopeClientID, "client-id", "", "target client-id (required)")
	clientsScopesAssignCmd.Flags().StringSliceVar(&scopeNames, "scope", nil, "client scope name(s) to assign (required)")
	clientsScopesAssignCmd.Flags().StringVar(&scopeType, "type", "default", "assignment type: default|optional")
	clientsScopesRemoveCmd.Flags().StringVar(&scopeClientID, "client-id", "", "target client-id (required)")
	clientsScopesRemoveCmd.Flags().StringSliceVar(&scopeNames, "scope", nil, "client scope name(s) to remove (required)")
	clientsScopesRemoveCmd.Flags().StringVar(&scopeType, "type", "default", "assignment type: default|optional")
	clientsScopesRemoveCmd.Flags().BoolVar(&scopeIgnoreMiss, "ignore-missing", false, "skip scopes not found/assigned instead of failing")

	// realm scope for all subcommands
	for _, c := range []*cobra.Command{clientsCreateCmd, clientsUpdateCmd, clientsDeleteCmd, clientsListCmd, clientsScopesAssignCmd, clientsScopesRemoveCmd} {
		c.Flags().StringSliceVar(&clientsRealms, "realm", nil, "target realm(s). If omitted, uses default or config.json")
		c.Flags().BoolVar(&clientsAllRealms, "all-realms", false, "apply to all realms")
	}

	// Normalize redirect-uri/web-origin into per-index slices during PreRun for create/update
	normalizeLists := func(cmd *cobra.Command) {
		if cmd.Flags().Changed("redirect-uri") {
			list, _ := cmd.Flags().GetStringSlice("redirect-uri")
			if len(list) > 0 {
				cliRedirectURIs = make([][]string, len(cliIDs))
				for i := range cliIDs {
					cliRedirectURIs[i] = append([]string{}, list...)
				}
			}
		}
		if cmd.Flags().Changed("web-origin") {
			list, _ := cmd.Flags().GetStringSlice("web-origin")
			if len(list) > 0 {
				cliWebOrigins = make([][]string, len(cliIDs))
				for i := range cliIDs {
					cliWebOrigins[i] = append([]string{}, list...)
				}
			}
		}
	}
	clientsCreateCmd.PreRun = func(cmd *cobra.Command, args []string) { normalizeLists(cmd) }
	clientsUpdateCmd.PreRun = func(cmd *cobra.Command, args []string) { normalizeLists(cmd) }
}
