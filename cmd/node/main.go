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
	LogLevel string
	NodeID   string
	Endpoint string
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
	rootCmd.PersistentFlags().StringVarP(&config.NodeID, "node-id", "n", os.Getenv("NODE_ID"),
		"Node ID (required)")

	var runCommand = &cobra.Command{
		Use:   "run",
		Short: "Run the NFS CSI driver",
		RunE: func(cmd *cobra.Command, args []string) error {
			driver := pkg.NewNodeServer(pkg.NodeCfg{
				Endpoint: config.Endpoint,
				NodeID:   config.NodeID,
			})
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

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
