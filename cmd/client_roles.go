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
	clientRolesNames        []string
	clientRolesDescriptions []string
	clientRolesAllRealms    bool
	clientRolesRealm        string
	clientRolesClientID     string
)

var clientRolesCmd = &cobra.Command{
	Use:   "client-roles",
	Short: "Manage client roles",
}

var clientRolesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create client role(s) in a client",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if clientRolesClientID == "" {
			return errors.New("missing --client-id: target client-id is required")
		}
		if len(clientRolesNames) == 0 {
			return errors.New("missing --name: provide at least one --name")
		}
		if !(len(clientRolesDescriptions) == 0 || len(clientRolesDescriptions) == 1 || len(clientRolesDescriptions) == len(clientRolesNames)) {
			return fmt.Errorf("invalid descriptions: when using multiple --name flags, you must pass either no --description, a single --description to apply to all, or one --description per --name (in order)")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		gc, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}

		var targetRealms []string
		if clientRolesAllRealms {
			realms, err := gc.GetRealms(ctx, token)
			if err != nil {
				return err
			}
			for _, r := range realms {
				if r.Realm != nil {
					targetRealms = append(targetRealms, *r.Realm)
				}
			}
		} else {
			r := clientRolesRealm
			if r == "" {
				r = defaultRealm
			}
			if r == "" {
				r = config.Global.Realm
			}
			if r == "" {
				return errors.New("target realm not specified. Use --realm or set realm in config.json")
			}
			targetRealms = []string{r}
		}

		created := 0
		skipped := 0
		var lines []string
		for _, realm := range targetRealms {
			c, err := getClientByClientID(ctx, gc, token, realm, clientRolesClientID)
			if err != nil || c == nil || c.ID == nil {
				return fmt.Errorf("client %q not found in realm %s", clientRolesClientID, realm)
			}
			clientID := *c.ID

			for i, rn := range clientRolesNames {
				_, err := gc.GetClientRole(ctx, token, realm, clientID, rn)
				if err == nil {
					lines = append(lines, fmt.Sprintf("Client role %q already exists in client %q (realm %q). Skipped.", rn, clientRolesClientID, realm))
					skipped++
					continue
				}
				if !strings.Contains(strings.ToLower(err.Error()), "404") {
					return fmt.Errorf("failed checking client role in client %s, realm %s: %w", clientRolesClientID, realm, err)
				}

				name := rn
				var desc string
				if len(clientRolesDescriptions) == 1 {
					desc = clientRolesDescriptions[0]
				} else if len(clientRolesDescriptions) == len(clientRolesNames) {
					desc = clientRolesDescriptions[i]
				} else {
					desc = ""
				}

				_, err = gc.CreateClientRole(ctx, token, realm, clientID, gocloak.Role{
					Name:        &name,
					Description: &desc,
				})
				if err != nil {
					return fmt.Errorf("failed creating client role %q in client %s, realm %s: %w", rn, clientRolesClientID, realm, err)
				}
				lines = append(lines, fmt.Sprintf("Created client role %q in client %q (realm %q).", rn, clientRolesClientID, realm))
				created++
			}
		}

		lines = append(lines, fmt.Sprintf("Done. Created: %d, Skipped: %d.", created, skipped))
		realmLabel := ""
		if clientRolesAllRealms {
			realmLabel = "all realms"
		} else if clientRolesRealm != "" {
			realmLabel = clientRolesRealm
		} else if len(targetRealms) == 1 {
			realmLabel = targetRealms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(clientRolesCmd)

	clientRolesCmd.AddCommand(clientRolesCreateCmd)
	clientRolesCreateCmd.Flags().StringVar(&clientRolesClientID, "client-id", "", "target client-id (required)")
	clientRolesCreateCmd.Flags().StringSliceVar(&clientRolesNames, "name", nil, "client role name(s). Repeatable; required.")
	clientRolesCreateCmd.Flags().StringSliceVar(&clientRolesDescriptions, "description", nil, "client role description(s). Pass none, one (applies to all), or one per --name in order.")
	clientRolesCreateCmd.Flags().BoolVar(&clientRolesAllRealms, "all-realms", false, "create client role in all realms")
	clientRolesCreateCmd.Flags().StringVar(&clientRolesRealm, "realm", "", "target realm")
}
