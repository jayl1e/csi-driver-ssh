package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jayl1e/csi-driver-ssh/pkg"
)

type Config struct {
	pkg.ControllerCfg
	LogLevel string
}

func validateConfig(config *Config) error {
	if config.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if config.CreateCmd == "" {
		return fmt.Errorf("create-script is required")
	}
	if config.DeleteCmd == "" {
		return fmt.Errorf("delete-script is required")
	}
	return nil
}

func main() {
	var config Config

	var rootCmd = &cobra.Command{
		Use:     "csi plugin for external nfs with shell",
		Short:   "nfs csi plugin that can run shell scripts hook",
		Version: pkg.DriverVersion,
	}
	rootCmd.PersistentFlags().StringVarP(&config.LogLevel, "log-level", "l", "info",
		"Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVarP(&config.CreateCmd, "create-cmd", "c", os.Getenv("CREATE_CMD"),
		"script to create volume")
	rootCmd.PersistentFlags().StringVarP(&config.DeleteCmd, "delete-cmd", "d", os.Getenv("DELETE_CMD"),
		"script to delete volume")
	rootCmd.PersistentFlags().StringVarP(&config.ExpandCmd, "expand-cmd", "", os.Getenv("EXPAND_CMD"),
		"script to expand volume")
	rootCmd.PersistentFlags().StringVarP(&config.CreateSnapshotCmd, "create-snapshot-cmd", "", os.Getenv("CREATE_SNAPSHOT_CMD"),
		"script to create snapshot")
	rootCmd.PersistentFlags().StringVarP(&config.DeleteSnapshotCmd, "delete-snapshot-cmd", "", os.Getenv("DELETE_SNAPSHOT_CMD"),
		"script to delete snapshot")
	rootCmd.PersistentFlags().StringVarP(&config.SSHConfig.SshServer, "ssh-server", "", os.Getenv("SSH_SERVER"),
		"SSH server address")
	rootCmd.PersistentFlags().StringVarP(&config.SSHConfig.SshUser, "ssh-user", "", os.Getenv("SSH_USER"),
		"SSH user")
	rootCmd.PersistentFlags().StringVarP(&config.SSHConfig.SshKey, "ssh-key", "", os.Getenv("SSH_KEY"),
		"SSH private key")

	var runCommand = &cobra.Command{
		Use:   "run",
		Short: "Run the NFS CSI driver",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateConfig(&config)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			driverCfg := config.ControllerCfg
			driver := pkg.NewController(driverCfg)
			var level slog.Level
			err := level.UnmarshalText([]byte(config.LogLevel))
			if err != nil {
				return fmt.Errorf("invalid log level: %w", err)
			}
			slog.SetLogLoggerLevel(level)
			return driver.Run()
		},
	}
	runCommand.Flags().StringVarP(&config.Endpoint, "endpoint", "e", "unix:///tmp/csi.sock",
		"CSI endpoint (e.g., unix:///tmp/csi.sock or tcp://0.0.0.0:9000)")
	rootCmd.AddCommand(runCommand)

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Host NFS CSI Plugin For Shell\n")
			fmt.Printf("Driver Name: %s\n", pkg.DriverName)
			fmt.Printf("Version: %s\n", pkg.DriverVersion)
		},
	}
	rootCmd.AddCommand(versionCmd)

	var validateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and scripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Validating configuration...")

			if err := validateConfig(&config); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}
			fmt.Println("✓ Configuration is valid")

			fmt.Println("All validations passed!")
			return nil
		},
	}
	rootCmd.AddCommand(validateCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
