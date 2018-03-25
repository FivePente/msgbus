package main

import (
	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/prologic/msgbus"
)

var configFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "msgbus",
	Version: msgbus.FullVersion(),
	Short:   "Command-line client for msgbus",
	Long: `This is the command-line client for the msgbus daemon msgbusd.

This lets you publish, subscribe and pull messages from a running msgbusd
instance. This is the reference implementation of using the msgbus client
library for publishing and subscribing to topics.`,
}

// Execute adds all child commands to the root command
// and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(
		&configFile, "config", "",
		"config file (default is $HOME/.msgbus.yaml)",
	)

	RootCmd.PersistentFlags().StringP(
		"uri", "u", "http://localhost:8000",
		"URI to connect to msgbusd",
	)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if configFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(configFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cobra-example" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".cobra-example")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
