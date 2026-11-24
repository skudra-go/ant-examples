package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd *cobra.Command

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func initRootCmd() {
	if RootCmd != nil {
		return
	}

	RootCmd = &cobra.Command{
		Use:   "server",
		Short: "Server",
		Long: `By default, server will start serving using the web server with no
  arguments - which can alternatively be run by running the subcommand web.`,
		RunE: runWeb,
	}
}

func loadDotEnv() error {
	if err := gotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}

	return nil
}

func main() {
	if err := loadDotEnv(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	initRootCmd()
	Execute()
}
