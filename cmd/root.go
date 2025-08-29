package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jsfinder",
	Short: "A CLI tool for crawling domains and extracting JavaScript files to find secrets and endpoints",
	Long: `jsfinder is a cybersecurity tool that crawls target domains,
extracts JavaScript files, and scans them for secrets, API keys, and hidden endpoints.

It provides three main commands:
- crawl: Crawl domains and extract JS files
- scan: Scan JS files for secrets and API keys
- discover: Brute-force endpoints using wordlists`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can be added here
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringP("config", "c", "", "Config file (default is ./config.yaml)")
}