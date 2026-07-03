package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	ModelDir string
	DataDir  string
}

var App *Config

// Initialize sets up viper and the default config paths.
func Initialize(cfgFile string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: unable to find home directory: %v\n", err)
		os.Exit(1)
	}

	defaultDataDir := filepath.Join(home, ".unbound")
	defaultModelDir := filepath.Join(defaultDataDir, "models")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(defaultDataDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetDefault("model_dir", defaultModelDir)
	viper.SetDefault("data_dir", defaultDataDir)

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		}
	}

	App = &Config{
		ModelDir: viper.GetString("model_dir"),
		DataDir:  viper.GetString("data_dir"),
	}

	// Ensure directories exist
	for _, d := range []string{App.DataDir, App.ModelDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating directory %s: %v\n", d, err)
		}
	}
}
