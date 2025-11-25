package cmd

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"

	"kc/internal/config"
	"kc/internal/keycloak"

	"github.com/Nerzal/gocloak/v13"
	"github.com/spf13/cobra"
)

var (
	usernames          []string
	emails             []string
	firstNames         []string
	lastNames          []string
	passwords          []string
	usersEnabled       bool
	usersRealms        []string
	usersAllRealms     bool
	realmRoleNames     []string
	clientRoleNames    []string
	clientRoleClientID string
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
		var passwordPairs []string
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

				// If no password provided, generate one automatically (fixed length 12)
				if pw == "" {
					generated, err := generateStrongPassword(12)
					if err != nil {
						return fmt.Errorf("failed generating password for user %q in realm %s: %w", un, realm, err)
					}
					pw = generated
					lines = append(lines, fmt.Sprintf("Generated password for user %q in realm %q.", un, realm))
				}

				// Validate password strength (provided or generated)
				if err := validatePasswordStrength(pw); err != nil {
					return fmt.Errorf("invalid password for user %q in realm %s: %w", un, realm, err)
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
				creds := []gocloak.CredentialRepresentation{{
					Type:      gocloak.StringP("password"),
					Value:     gocloak.StringP(pw),
					Temporary: gocloak.BoolP(false),
				}}
				user.Credentials = &creds

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
				// Assign client roles if requested
				if len(clientRoleNames) > 0 {
					if clientRoleClientID == "" {
						return errors.New("missing --client-id when using --client-role")
					}
					kcClient, err := getClientByClientID(ctx, client, token, realm, clientRoleClientID)
					if err != nil || kcClient == nil || kcClient.ID == nil {
						return fmt.Errorf("client %q not found in realm %s", clientRoleClientID, realm)
					}
					idOfClient := *kcClient.ID
					var roles []gocloak.Role
					for _, rn := range clientRoleNames {
						role, err := client.GetClientRole(ctx, token, realm, idOfClient, rn)
						if err != nil {
							return fmt.Errorf("failed fetching client role %q for client %s in realm %s: %w", rn, clientRoleClientID, realm, err)
						}
						roles = append(roles, *role)
					}
					if err := client.AddClientRoleToUser(ctx, token, realm, idOfClient, userID, roles); err != nil {
						return fmt.Errorf("failed assigning client roles to user %q in realm %s: %w", un, realm, err)
					}
				}

				lines = append(lines, fmt.Sprintf("Created user %q (ID: %s) in realm %q.", un, userID, realm))
				lines = append(lines, fmt.Sprintf("Password for user %q in realm %q: %s", un, realm, pw))
				passwordPairs = append(passwordPairs, fmt.Sprintf("%s=%s@%s", un, pw, realm))
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
		if len(passwordPairs) > 0 {
			auditDetails = "passwords: " + strings.Join(passwordPairs, ", ")
		}
		printBox(cmd, lines, realmLabel)
		return nil
	}),
}

func validatePasswordStrength(pw string) error {
	// User-provided (or generated) passwords must be at least 6 characters long
	if len(pw) < 6 {
		return fmt.Errorf("password must be at least 6 characters long")
	}
	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			// Anything that is not a letter or digit is considered special
			hasSpecial = true
		}
	}
	if !hasLower || !hasUpper || !hasDigit || !hasSpecial {
		return errors.New("password must contain at least one lowercase letter, one uppercase letter, one digit, and one special character")
	}
	return nil
}

func generateStrongPassword(n int) (string, error) {
	const lower = "abcdefghijklmnopqrstuvwxyz"
	const upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digits = "0123456789"
	const specials = "!@#$%^&*()-_=+[]{}|;:,.<>/?"
	const all = lower + upper + digits + specials

	// We need at least one of each type: lower, upper, digit, special
	if n < 4 {
		return "", errors.New("password length must be at least 4")
	}

	b := make([]byte, n)

	// ensure at least one of each required type
	pools := []string{lower, upper, digits, specials}
	for i, pool := range pools {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(pool))))
		if err != nil {
			return "", err
		}
		b[i] = pool[idx.Int64()]
	}

	for i := len(pools); i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(all))))
		if err != nil {
			return "", err
		}
		b[i] = all[idx.Int64()]
	}

	return string(b), nil
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
		var passwordPairs []string
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

				if pw != "" {
					if err := validatePasswordStrength(pw); err != nil {
						return fmt.Errorf("invalid password for user %q in realm %s: %w", un, realm, err)
					}
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
					lines = append(lines, fmt.Sprintf("Updated password for user %q in realm %q.", un, realm))
					lines = append(lines, fmt.Sprintf("New password for user %q in realm %q: %s", un, realm, pw))
					passwordPairs = append(passwordPairs, fmt.Sprintf("%s=%s@%s", un, pw, realm))
				}
				lines = append(lines, fmt.Sprintf("Updated user %q (ID: %s) in realm %q.", un, userID, realm))
				updated++
			}
		}
		lines = append(lines, fmt.Sprintf("Done. Updated: %d, Skipped: %d.", updated, skipped))
		if len(passwordPairs) > 0 {
			auditDetails = "passwords: " + strings.Join(passwordPairs, ", ")
		}
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
	usersCreateCmd.Flags().StringSliceVar(&clientRoleNames, "client-role", nil, "client role name(s) to assign to each created user")
	usersCreateCmd.Flags().StringVar(&clientRoleClientID, "client-id", "", "client-id whose roles will be assigned to created users")

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
