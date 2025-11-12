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
	roleNames        []string
	roleDescriptions []string
	allRealms        bool
	rolesRealm       string
	newRoleNames     []string
	ignoreMissing    bool
	ignoreMissingDel bool
)

var rolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "Manage roles",
}

var rolesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a role in a realm or in all realms",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(roleNames) == 0 {
			return errors.New("missing --name: provide at least one --name")
		}
		// Validate descriptions: allowed counts are 0, 1, or exactly the number of names
		if !(len(roleDescriptions) == 0 || len(roleDescriptions) == 1 || len(roleDescriptions) == len(roleNames)) {
			return fmt.Errorf("invalid descriptions: when using multiple --name flags, you must pass either no --description, a single --description to apply to all, or one --description per --name (in order)")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}
		var targetRealms []string
		if allRealms {
			realms, err := client.GetRealms(ctx, token)
			if err != nil {
				return err
			}
			for _, r := range realms {
				if r.Realm != nil {
					targetRealms = append(targetRealms, *r.Realm)
				}
			}
		} else {
			r := rolesRealm
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
		for _, realm := range targetRealms {
			for i, rn := range roleNames {
				exists := false
				_, err := client.GetRealmRole(ctx, token, realm, rn)
				if err == nil {
					exists = true
				} else {
					if !strings.Contains(strings.ToLower(err.Error()), "404") {
						return fmt.Errorf("failed checking role in realm %s: %w", realm, err)
					}
				}
				if exists {
					fmt.Fprintf(cmd.OutOrStdout(), "Role %q already exists in realm %q. Skipped.\n", rn, realm)
					skipped++
					continue
				}
				name := rn
				var desc string
				if len(roleDescriptions) == 1 {
					desc = roleDescriptions[0]
				} else if len(roleDescriptions) == len(roleNames) {
					desc = roleDescriptions[i]
				} else {
					desc = ""
				}
				_, err = client.CreateRealmRole(ctx, token, realm, gocloak.Role{
					Name:        &name,
					Description: &desc,
				})
				if err != nil {
					return fmt.Errorf("failed creating role %q in realm %s: %w", rn, realm, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created role %q in realm %q.\n", rn, realm)
				created++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Done. Created: %d, Skipped: %d.\n", created, skipped)
		return nil
	}),
}

var rolesUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update role(s) in a realm or across realms",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(roleNames) == 0 {
			return errors.New("missing --name: provide at least one --name")
		}
		// At least one of description or new-name must be provided
		if len(roleDescriptions) == 0 && len(newRoleNames) == 0 {
			return errors.New("nothing to update: provide --description and/or --new-name")
		}
		// Validate counts for description and new-name: 0, 1, or len(names)
		if !(len(roleDescriptions) == 0 || len(roleDescriptions) == 1 || len(roleDescriptions) == len(roleNames)) {
			return fmt.Errorf("invalid descriptions: pass none, one (applies to all), or one per --name (in order)")
		}
		if !(len(newRoleNames) == 0 || len(newRoleNames) == 1 || len(newRoleNames) == len(roleNames)) {
			return fmt.Errorf("invalid new names: pass none, one (applies to all), or one per --name (in order)")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}

		var targetRealms []string
		if allRealms {
			realms, err := client.GetRealms(ctx, token)
			if err != nil {
				return err
			}
			for _, r := range realms {
				if r.Realm != nil {
					targetRealms = append(targetRealms, *r.Realm)
				}
			}
		} else {
			r := rolesRealm
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

		updated := 0
		skipped := 0
		for _, realm := range targetRealms {
			for i, rn := range roleNames {
				role, err := client.GetRealmRole(ctx, token, realm, rn)
				if err != nil {
					// 404 handling
					if strings.Contains(strings.ToLower(err.Error()), "404") {
						if ignoreMissing {
							fmt.Fprintf(cmd.OutOrStdout(), "Role %q not found in realm %q. Skipped.\n", rn, realm)
							skipped++
							continue
						}
						return fmt.Errorf("role %q not found in realm %s", rn, realm)
					}
					return fmt.Errorf("failed fetching role %q in realm %s: %w", rn, realm, err)
				}
				// Apply changes
				if len(roleDescriptions) == 1 {
					role.Description = &roleDescriptions[0]
				} else if len(roleDescriptions) == len(roleNames) {
					role.Description = &roleDescriptions[i]
				}
				if len(newRoleNames) == 1 {
					role.Name = &newRoleNames[0]
				} else if len(newRoleNames) == len(roleNames) {
					role.Name = &newRoleNames[i]
				}
				if err := client.UpdateRealmRole(ctx, token, realm, rn, *role); err != nil {
					return fmt.Errorf("failed updating role %q in realm %s: %w", rn, realm, err)
				}
				finalName := rn
				if role.Name != nil {
					finalName = *role.Name
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Updated role %q in realm %q. New name: %q.\n", rn, realm, finalName)
				updated++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Done. Updated: %d, Skipped: %d.\n", updated, skipped)
		return nil
	}),
}

var rolesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete role(s) in a realm or across realms",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		if len(roleNames) == 0 {
			return errors.New("missing --name: provide at least one --name")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		client, token, err := keycloak.Login(ctx)
		if err != nil {
			return err
		}

		var targetRealms []string
		if allRealms {
			realms, err := client.GetRealms(ctx, token)
			if err != nil {
				return err
			}
			for _, r := range realms {
				if r.Realm != nil {
					targetRealms = append(targetRealms, *r.Realm)
				}
			}
		} else {
			r := rolesRealm
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

		deleted := 0
		skipped := 0
		for _, realm := range targetRealms {
			for _, rn := range roleNames {
				if err := client.DeleteRealmRole(ctx, token, realm, rn); err != nil {
					if strings.Contains(strings.ToLower(err.Error()), "404") {
						if ignoreMissingDel {
							fmt.Fprintf(cmd.OutOrStdout(), "Role %q not found in realm %q. Skipped.\n", rn, realm)
							skipped++
							continue
						}
						return fmt.Errorf("role %q not found in realm %s", rn, realm)
					}
					return fmt.Errorf("failed deleting role %q in realm %s: %w", rn, realm, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted role %q in realm %q.\n", rn, realm)
				deleted++
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Done. Deleted: %d, Skipped: %d.\n", deleted, skipped)
		return nil
	}),
}

func init() {
	rootCmd.AddCommand(rolesCmd)
	rolesCmd.AddCommand(rolesCreateCmd)
	rolesCreateCmd.Flags().StringSliceVar(&roleNames, "name", nil, "role name(s). You can repeat --name multiple times.")
	rolesCreateCmd.Flags().StringSliceVar(&roleDescriptions, "description", nil, "role description(s). Pass none, one (applies to all), or one per --name in order.")
	rolesCreateCmd.Flags().BoolVar(&allRealms, "all-realms", false, "create role in all realms")
	rolesCreateCmd.Flags().StringVar(&rolesRealm, "realm", "", "target realm")

	rolesCmd.AddCommand(rolesUpdateCmd)
	rolesUpdateCmd.Flags().StringSliceVar(&roleNames, "name", nil, "role name(s) to update. Repeatable; required.")
	rolesUpdateCmd.Flags().StringSliceVar(&roleDescriptions, "description", nil, "new description(s). Pass none, one (applies to all), or one per --name in order.")
	rolesUpdateCmd.Flags().StringSliceVar(&newRoleNames, "new-name", nil, "new role name(s). Pass none, one (applies to all), or one per --name in order.")
	rolesUpdateCmd.Flags().BoolVar(&allRealms, "all-realms", false, "update role(s) in all realms")
	rolesUpdateCmd.Flags().StringVar(&rolesRealm, "realm", "", "target realm")
	rolesUpdateCmd.Flags().BoolVar(&ignoreMissing, "ignore-missing", false, "skip roles not found instead of failing")

	rolesCmd.AddCommand(rolesDeleteCmd)
	rolesDeleteCmd.Flags().StringSliceVar(&roleNames, "name", nil, "role name(s) to delete. Repeatable; required.")
	rolesDeleteCmd.Flags().BoolVar(&allRealms, "all-realms", false, "delete role(s) in all realms")
	rolesDeleteCmd.Flags().StringVar(&rolesRealm, "realm", "", "target realm")
	rolesDeleteCmd.Flags().BoolVar(&ignoreMissingDel, "ignore-missing", false, "skip roles not found instead of failing")
}
