package cmd

import (
    "context"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "kc/internal/config"

    "github.com/spf13/cobra"
)

var (
    cfgFile      string
    defaultRealm string
    logFile      string
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
            fmt.Fprintf(cmd.ErrOrStderr(), "[%s] END: status=ok dur=%s\n", end.Format(time.RFC3339), dur)
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
            fmt.Fprintf(cmd.ErrOrStderr(), "[%s] END: status=error dur=%s\n", end.Format(time.RFC3339), dur)
            ctx := context.WithValue(cmd.Context(), ctxKeyEnded{}, true)
            cmd.SetContext(ctx)
        }
        return err
    }
}
