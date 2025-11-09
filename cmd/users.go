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
	usernames      []string
	emails         []string
	firstNames     []string
	lastNames      []string
	passwords      []string
	usersEnabled   bool
	usersRealms    []string
	usersAllRealms bool
	realmRoleNames []string
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage users",
}

var usersCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create user(s) in one or multiple realms",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(usernames) == 0 {
			return errors.New("missing --username: provide at least one --username")
		}
		// Validate optional per-user slices: allowed counts are 0, 1, or equal to usernames
		validateSlice := func(name string, n int) error {
			if !(n == 0 || n == 1 || n == len(usernames)) {
				return fmt.Errorf("invalid %s: when using multiple --username, you must pass either no %s, a single %s to apply to all, or one %s per --username (in order)", name, name, name, name)
			}
			return nil
		}
		if err := validateSlice("--email", len(emails)); err != nil {
			return err
		}
		if err := validateSlice("--first-name", len(firstNames)); err != nil {
			return err
		}
		if err := validateSlice("--last-name", len(lastNames)); err != nil {
			return err
		}
		if err := validateSlice("--password", len(passwords)); err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		client, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}

		// Resolve target realms
		var targetRealms []string
		if usersAllRealms {
			realms, err := client.GetRealms(ctx, token)
			if err != nil {
				return err
			}
			for _, r := range realms {
				if r.Realm != nil {
					targetRealms = append(targetRealms, *r.Realm)
				}
			}
		} else if len(usersRealms) > 0 {
			targetRealms = append(targetRealms, usersRealms...)
		} else {
			r := defaultRealm
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
		for _, realm := range targetRealms {
			for i, un := range usernames {
				// Lookup existence by username
				params := gocloak.GetUsersParams{Username: &un}
				existing, err := client.GetUsers(ctx, token, realm, params)
				if err != nil {
					return fmt.Errorf("failed searching user %q in realm %s: %w", un, realm, err)
				}
				if len(existing) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "User %q already exists in realm %q. Skipped.\n", un, realm)
					skipped++
					continue
				}

				var em, fn, ln, pw string
				if len(emails) == 1 {
					em = emails[0]
				} else if len(emails) == len(usernames) {
					em = emails[i]
				}
				if len(firstNames) == 1 {
					fn = firstNames[0]
				} else if len(firstNames) == len(usernames) {
					fn = firstNames[i]
				}
				if len(lastNames) == 1 {
					ln = lastNames[0]
				} else if len(lastNames) == len(usernames) {
					ln = lastNames[i]
				}
				if len(passwords) == 1 {
					pw = passwords[0]
				} else if len(passwords) == len(usernames) {
					pw = passwords[i]
				}

				enabled := usersEnabled
				emailVerified := em != ""

				user := gocloak.User{
					Username:      &un,
					Enabled:       &enabled,
					EmailVerified: &emailVerified,
				}
				if em != "" {
					user.Email = &em
				}
				if fn != "" {
					user.FirstName = &fn
				}
				if ln != "" {
					user.LastName = &ln
				}
				if pw != "" {
					creds := []gocloak.CredentialRepresentation{{
						Type:      gocloak.StringP("password"),
						Value:     gocloak.StringP(pw),
						Temporary: gocloak.BoolP(false),
					}}
					user.Credentials = &creds
				}

				userID, err := client.CreateUser(ctx, token, realm, user)
				if err != nil {
					// Surfacing 409 conflicts more nicely
					if strings.Contains(strings.ToLower(err.Error()), "409") {
						fmt.Fprintf(cmd.OutOrStdout(), "User %q already exists in realm %q. Skipped.\n", un, realm)
						skipped++
						continue
					}
					return fmt.Errorf("failed creating user %q in realm %s: %w", un, realm, err)
				}

				// Assign realm roles if requested
				if len(realmRoleNames) > 0 {
					var roles []gocloak.Role
					for _, rn := range realmRoleNames {
						role, err := client.GetRealmRole(ctx, token, realm, rn)
						if err != nil {
							return fmt.Errorf("failed fetching realm role %q in realm %s: %w", rn, realm, err)
						}
						roles = append(roles, *role)
					}
					if err := client.AddRealmRoleToUser(ctx, token, realm, userID, roles); err != nil {
						return fmt.Errorf("failed assigning roles to user %q in realm %s: %w", un, realm, err)
					}
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Created user %q (ID: %s) in realm %q.\n", un, userID, realm)
				created++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Done. Created: %d, Skipped: %d.\n", created, skipped)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(usersCmd)
	usersCmd.AddCommand(usersCreateCmd)
	usersCreateCmd.Flags().StringSliceVar(&usernames, "username", nil, "username(s). Repeatable; required.")
	usersCreateCmd.Flags().StringSliceVar(&emails, "email", nil, "email(s). Optional; 0, 1 or N matching --username.")
	usersCreateCmd.Flags().StringSliceVar(&firstNames, "first-name", nil, "first name(s). Optional; 0, 1 or N matching --username.")
	usersCreateCmd.Flags().StringSliceVar(&lastNames, "last-name", nil, "last name(s). Optional; 0, 1 or N matching --username.")
	usersCreateCmd.Flags().StringSliceVar(&passwords, "password", nil, "password(s). Optional; 0, 1 or N matching --username.")
	usersCreateCmd.Flags().BoolVar(&usersEnabled, "enabled", true, "whether the user(s) are enabled; defaults to true")
	usersCreateCmd.Flags().StringSliceVar(&usersRealms, "realm", nil, "target realm(s). If omitted, uses default or config.json")
	usersCreateCmd.Flags().BoolVar(&usersAllRealms, "all-realms", false, "create users in all realms")
	usersCreateCmd.Flags().StringSliceVar(&realmRoleNames, "realm-role", nil, "realm role name(s) to assign to each created user")
}
