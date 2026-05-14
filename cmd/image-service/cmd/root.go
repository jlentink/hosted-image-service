package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	BuildDate = "unknown"
	cfgFile   string
)

var rootCmd = &cobra.Command{
	Use:   "image-service",
	Short: "High-performance image resize and crop service",
	Long:  "A self-hostable Go service that resizes and crops images with support for JPEG, PNG, WebP, and AVIF formats.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (default: ./config.toml)")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("image-service %s (built %s)\n", Version, BuildDate)
	},
}

func getConfigFile() string {
	if cfgFile != "" {
		return cfgFile
	}
	if env := os.Getenv("IMAGE_SERVICE_CONFIG"); env != "" {
		return env
	}
	return ""
}
