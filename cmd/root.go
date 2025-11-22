package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"kc/internal/audit"
	"kc/internal/config"
	"kc/internal/ui"

	"github.com/spf13/cobra"
)

var (
	cfgFile      string
	defaultRealm string
	logFile      string
	jiraTicket   string
	auditDetails string
)

var rootCmd = &cobra.Command{
	Use:   "kc",
	Short: "Keycloak CLI",
	Long:  "Keycloak CLI",
	RunE: withErrorEnd(func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "Keycloak CLI")
		fmt.Fprintln(cmd.OutOrStdout(), "")
		return cmd.Help()
	}),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Load(cfgFile); err != nil {
			return err
		}
		if err := setupTeeWriters(cmd); err != nil {
			return err
		}
		start := time.Now()
		raw := buildRawCommand()
		fmt.Fprintf(cmd.ErrOrStderr(), "[%s] START: %s\n", start.Format(time.RFC3339), raw)
		ctx := context.WithValue(cmd.Context(), ctxKeyStart{}, start)
		ctx = context.WithValue(ctx, ctxKeyEnded{}, false)
		cmd.SetContext(ctx)
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		ended, _ := cmd.Context().Value(ctxKeyEnded{}).(bool)
		if !ended {
			start, _ := cmd.Context().Value(ctxKeyStart{}).(time.Time)
			end := time.Now()
			dur := end.Sub(start)
			fmt.Fprintf(cmd.ErrOrStderr(), "[%s] END: status=ok dur=%s\n\n", end.Format(time.RFC3339), dur)
			appendAudit(cmd, "ok", start, end, dur)
		}
		if logDest != nil {
			_ = logDest.Close()
			logDest = nil
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
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "kc.log", "path to the log file")
	rootCmd.PersistentFlags().StringVar(&jiraTicket, "jira", "", "Jira ticket identifier for display in command output")
}

type ctxKeyStart struct{}
type ctxKeyEnded struct{}

var logDest io.WriteCloser

func setupTeeWriters(cmd *cobra.Command) error {
	lf := logFile
	if lf == "" {
		lf = "kc.log"
	}
	f, err := os.OpenFile(lf, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	logDest = f
	out := io.MultiWriter(cmd.OutOrStdout(), f)
	errw := io.MultiWriter(cmd.ErrOrStderr(), f)
	cmd.SetOut(out)
	cmd.SetErr(errw)
	return nil
}

func buildRawCommand() string {
	if len(os.Args) == 0 {
		return "./kc.exe"
	}
	return "./kc.exe " + strings.Join(os.Args[1:], " ")
}

func withErrorEnd(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := run(cmd, args)
		if err != nil {
			start, _ := cmd.Context().Value(ctxKeyStart{}).(time.Time)
			end := time.Now()
			dur := end.Sub(start)
			fmt.Fprintf(cmd.ErrOrStderr(), "[%s] ERROR: %v\n", end.Format(time.RFC3339), err)
			fmt.Fprintf(cmd.ErrOrStderr(), "[%s] END: status=error dur=%s\n\n", end.Format(time.RFC3339), dur)
			appendAudit(cmd, "error", start, end, dur)
			ctx := context.WithValue(cmd.Context(), ctxKeyEnded{}, true)
			cmd.SetContext(ctx)
		}
		return err
	}
}

func printBox(cmd *cobra.Command, lines []string, realmLabel string) {
	opts := ui.BoxOptions{
		JiraTicket: jiraTicket,
		Realm:      realmLabel,
		Title:      "Keycloak CLI",
	}
	box := ui.RenderBox(lines, opts)
	fmt.Fprintln(cmd.OutOrStdout(), box)
}

func appendAudit(cmd *cobra.Command, status string, start, end time.Time, dur time.Duration) {
	raw := buildRawCommand()
	actorType, actorID := resolveActor()
	targetRealms := resolveTargetRealms()
	changeKind := resolveChangeKind(cmd.CommandPath())
	entry := audit.Entry{
		Timestamp:    end,
		Status:       status,
		CommandPath:  cmd.CommandPath(),
		RawCommand:   raw,
		Jira:         jiraTicket,
		ActorType:    actorType,
		ActorID:      actorID,
		AuthRealm:    config.Global.AuthRealm,
		ChangeKind:   changeKind,
		TargetRealms: targetRealms,
		Duration:     dur.String(),
		Details:      auditDetails,
	}
	_ = audit.Append(entry)
	auditDetails = ""
}

func resolveActor() (string, string) {
	if config.Global.GrantType == "password" && config.Global.Username != "" {
		return "user", config.Global.Username
	}
	if config.Global.ClientID != "" {
		return "client", config.Global.ClientID
	}
	return "unknown", ""
}

func resolveTargetRealms() string {
	if defaultRealm != "" {
		return defaultRealm
	}
	if config.Global.Realm != "" {
		return config.Global.Realm
	}
	return ""
}

func resolveChangeKind(path string) string {
	switch path {
	case "kc users create":
		return "users_create"
	case "kc users update":
		return "users_update"
	case "kc users delete":
		return "users_delete"
	case "kc clients create":
		return "clients_create"
	case "kc clients update":
		return "clients_update"
	case "kc clients delete":
		return "clients_delete"
	case "kc clients list":
		return "clients_list"
	case "kc client-scopes create":
		return "client_scopes_create"
	case "kc client-scopes update":
		return "client_scopes_update"
	case "kc client-scopes delete":
		return "client_scopes_delete"
	case "kc client-scopes list":
		return "client_scopes_list"
	case "kc roles create":
		return "roles_create"
	case "kc roles update":
		return "roles_update"
	case "kc roles delete":
		return "roles_delete"
	case "kc realms list":
		return "realms_list"
	default:
		return path
	}
}
