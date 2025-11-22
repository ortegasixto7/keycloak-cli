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
	// update-specific
	updEmails     []string
	updFirstNames []string
	updLastNames  []string
	updPasswords  []string
	updEnabled    bool
	updIgnoreMiss bool
	delIgnoreMiss bool
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage users",
}

var usersCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create user(s) in one or multiple realms",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
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
		var lines []string
		for _, realm := range targetRealms {
			for i, un := range usernames {
				// Lookup existence by username
				params := gocloak.GetUsersParams{Username: &un}
				existing, err := client.GetUsers(ctx, token, realm, params)
				if err != nil {
					return fmt.Errorf("failed searching user %q in realm %s: %w", un, realm, err)
				}
				if len(existing) > 0 {
					lines = append(lines, fmt.Sprintf("User %q already exists in realm %q. Skipped.", un, realm))
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

				lines = append(lines, fmt.Sprintf("Created user %q (ID: %s) in realm %q.", un, userID, realm))
				created++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Created: %d, Skipped: %d.", created, skipped))
		realmLabel := ""
		if usersAllRealms {
			realmLabel = "all realms"
		} else if len(usersRealms) == 1 {
			realmLabel = usersRealms[0]
		} else if len(targetRealms) == 1 {
			realmLabel = targetRealms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

var usersUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update user(s) in one or multiple realms",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(usernames) == 0 {
			return errors.New("missing --username: provide at least one --username")
		}
		// Determine if enabled flag was provided
		enabledChanged := cmd.Flags().Changed("enabled")

		// Must have at least one field to update
		if len(updEmails) == 0 && len(updFirstNames) == 0 && len(updLastNames) == 0 && len(updPasswords) == 0 && !enabledChanged {
			return errors.New("nothing to update: provide at least one of --email/--first-name/--last-name/--password/--enabled")
		}
		// Validate 0/1/N for provided slices
		validate := func(name string, n int) error {
			if !(n == 0 || n == 1 || n == len(usernames)) {
				return fmt.Errorf("invalid %s: when using multiple --username, pass none, one (applies to all), or one per --username (in order)", name)
			}
			return nil
		}
		if err := validate("--email", len(updEmails)); err != nil {
			return err
		}
		if err := validate("--first-name", len(updFirstNames)); err != nil {
			return err
		}
		if err := validate("--last-name", len(updLastNames)); err != nil {
			return err
		}
		if err := validate("--password", len(updPasswords)); err != nil {
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

		updated := 0
		skipped := 0
		var lines []string
		for _, realm := range targetRealms {
			for i, un := range usernames {
				params := gocloak.GetUsersParams{Username: &un}
				existing, err := client.GetUsers(ctx, token, realm, params)
				if err != nil {
					return fmt.Errorf("failed searching user %q in realm %s: %w", un, realm, err)
				}
				if len(existing) == 0 {
					if updIgnoreMiss {
						lines = append(lines, fmt.Sprintf("User %q not found in realm %q. Skipped.", un, realm))
						skipped++
						continue
					}
					return fmt.Errorf("user %q not found in realm %s", un, realm)
				}
				userID := *existing[0].ID

				var em, fn, ln, pw string
				if len(updEmails) == 1 {
					em = updEmails[0]
				} else if len(updEmails) == len(usernames) {
					em = updEmails[i]
				}
				if len(updFirstNames) == 1 {
					fn = updFirstNames[0]
				} else if len(updFirstNames) == len(usernames) {
					fn = updFirstNames[i]
				}
				if len(updLastNames) == 1 {
					ln = updLastNames[0]
				} else if len(updLastNames) == len(usernames) {
					ln = updLastNames[i]
				}
				if len(updPasswords) == 1 {
					pw = updPasswords[0]
				} else if len(updPasswords) == len(usernames) {
					pw = updPasswords[i]
				}

				u := gocloak.User{ID: &userID}
				if em != "" {
					u.Email = &em
					ev := true
					u.EmailVerified = &ev
				}
				if fn != "" {
					u.FirstName = &fn
				}
				if ln != "" {
					u.LastName = &ln
				}
				if enabledChanged {
					u.Enabled = &updEnabled
				}

				if err := client.UpdateUser(ctx, token, realm, u); err != nil {
					return fmt.Errorf("failed updating user %q in realm %s: %w", un, realm, err)
				}
				if pw != "" {
					if err := client.SetPassword(ctx, token, userID, realm, pw, false); err != nil {
						return fmt.Errorf("failed setting password for user %q in realm %s: %w", un, realm, err)
					}
				}
				lines = append(lines, fmt.Sprintf("Updated user %q (ID: %s) in realm %q.", un, userID, realm))
				updated++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Updated: %d, Skipped: %d.", updated, skipped))
		realmLabel := ""
		if usersAllRealms {
			realmLabel = "all realms"
		} else if len(usersRealms) == 1 {
			realmLabel = usersRealms[0]
		} else if len(targetRealms) == 1 {
			realmLabel = targetRealms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

var usersDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete user(s) in one or multiple realms",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(usernames) == 0 {
			return errors.New("missing --username: provide at least one --username")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		client, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}

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

		deleted := 0
		skipped := 0
		var lines []string
		for _, realm := range targetRealms {
			for _, un := range usernames {
				params := gocloak.GetUsersParams{Username: &un}
				existing, err := client.GetUsers(ctx, token, realm, params)
				if err != nil {
					return fmt.Errorf("failed searching user %q in realm %s: %w", un, realm, err)
				}
				if len(existing) == 0 {
					if delIgnoreMiss {
						lines = append(lines, fmt.Sprintf("User %q not found in realm %q. Skipped.", un, realm))
						skipped++
						continue
					}
					return fmt.Errorf("user %q not found in realm %s", un, realm)
				}
				userID := *existing[0].ID
				if err := client.DeleteUser(ctx, token, realm, userID); err != nil {
					return fmt.Errorf("failed deleting user %q in realm %s: %w", un, realm, err)
				}
				lines = append(lines, fmt.Sprintf("Deleted user %q (ID: %s) in realm %q.", un, userID, realm))
				deleted++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Deleted: %d, Skipped: %d.", deleted, skipped))
		realmLabel := ""
		if usersAllRealms {
			realmLabel = "all realms"
		} else if len(usersRealms) == 1 {
			realmLabel = usersRealms[0]
		} else if len(targetRealms) == 1 {
			realmLabel = targetRealms[0]
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
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

	usersCmd.AddCommand(usersUpdateCmd)
	usersUpdateCmd.Flags().StringSliceVar(&usernames, "username", nil, "username(s) to update. Repeatable; required.")
	usersUpdateCmd.Flags().StringSliceVar(&updEmails, "email", nil, "new email(s). Optional; 0, 1 or N matching --username.")
	usersUpdateCmd.Flags().StringSliceVar(&updFirstNames, "first-name", nil, "new first name(s). Optional; 0, 1 or N.")
	usersUpdateCmd.Flags().StringSliceVar(&updLastNames, "last-name", nil, "new last name(s). Optional; 0, 1 or N.")
	usersUpdateCmd.Flags().StringSliceVar(&updPasswords, "password", nil, "new password(s). Optional; 0, 1 or N.")
	usersUpdateCmd.Flags().BoolVar(&updEnabled, "enabled", true, "set enabled state for users; if flag is present, applies to all or per-user via 0/1/N not supported")
	usersUpdateCmd.Flags().StringSliceVar(&usersRealms, "realm", nil, "target realm(s). If omitted, uses default or config.json")
	usersUpdateCmd.Flags().BoolVar(&usersAllRealms, "all-realms", false, "update users in all realms")
	usersUpdateCmd.Flags().BoolVar(&updIgnoreMiss, "ignore-missing", false, "skip users not found instead of failing")

	usersCmd.AddCommand(usersDeleteCmd)
	usersDeleteCmd.Flags().StringSliceVar(&usernames, "username", nil, "username(s) to delete. Repeatable; required.")
	usersDeleteCmd.Flags().StringSliceVar(&usersRealms, "realm", nil, "target realm(s). If omitted, uses default or config.json")
	usersDeleteCmd.Flags().BoolVar(&usersAllRealms, "all-realms", false, "delete users in all realms")
	usersDeleteCmd.Flags().BoolVar(&delIgnoreMiss, "ignore-missing", false, "skip users not found instead of failing")
}
