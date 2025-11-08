package cmd

import (
	"fmt"
	"os"

	"kc/internal/config"

	"github.com/spf13/cobra"
)

var (
	cfgFile      string
	defaultRealm string
)

var rootCmd = &cobra.Command{
	Use:   "kc",
	Short: "Keycloak CLI",
	Long:  "Keycloak CLI",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "Keycloak CLI")
		fmt.Fprintln(cmd.OutOrStdout(), "")
		return cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Load(cfgFile); err != nil {
			return err
		}
		return nil
	},
}

func Execute() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (default: config.json next to the binary or current directory)")
	rootCmd.PersistentFlags().StringVar(&defaultRealm, "realm", "", "target realm")
}
