package cmd

import (
	"fmt"
	"os"

	"github.com/k0u3h1k/bare-metal/cmd/list"
	"github.com/k0u3h1k/bare-metal/cmd/pull"
	"github.com/k0u3h1k/bare-metal/cmd/remove"
	"github.com/k0u3h1k/bare-metal/cmd/run"
	"github.com/k0u3h1k/bare-metal/cmd/serve"
	"github.com/k0u3h1k/bare-metal/internal/config"
	"github.com/spf13/cobra"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "unbound",
	Short: "Run open-source AI models locally with full system access",
	Long: `Unbound is a desktop/local tool that downloads, hosts, and runs
open-source AI models entirely on your machine — no cloud, no censorship,
no restrictions. Models can be given full system permissions (shell access,
file I/O, code execution, internet) with your explicit consent.`,
	Version: "0.1.0-dev",
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.unbound/config.yaml)")
	rootCmd.PersistentFlags().String("model-dir", "", "directory to store downloaded models (default is $HOME/.unbound/models)")

	rootCmd.AddCommand(run.Cmd)
	rootCmd.AddCommand(serve.Cmd)
	rootCmd.AddCommand(pull.Cmd)
	rootCmd.AddCommand(list.Cmd)
	rootCmd.AddCommand(remove.Cmd)
}

func initConfig() {
	config.Initialize(cfgFile)
}
