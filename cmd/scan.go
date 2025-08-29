package cmd

import (
	"github.com/spf13/cobra"
	"jsfinder/pkg/scanner"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan JavaScript files for secrets and API keys",
	Long: `Scan JavaScript files for secrets, API keys, tokens, and other sensitive information.
Supports both file input and stdin for batch processing.`,
	Example: `  jsfinder scan --input jsfiles.txt --output secrets.json
  cat jsfiles.txt | jsfinder scan --output secrets.json`,
	RunE: runScan,
}

var (
	scanInputFile  string
	scanOutputFile string
	scanThreads    int
	scanTimeout    int
	configFile     string
	format         string
)

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVarP(&scanInputFile, "input", "i", "", "Input file containing JS file URLs")
	scanCmd.Flags().StringVarP(&scanOutputFile, "output", "o", "", "Output file for scan results")
	scanCmd.Flags().IntVarP(&scanThreads, "threads", "t", 10, "Number of concurrent threads")
	scanCmd.Flags().IntVarP(&scanTimeout, "timeout", "", 30, "Request timeout in seconds")
	scanCmd.Flags().StringVarP(&configFile, "config", "c", "", "Config file with regex patterns")
	scanCmd.Flags().StringVarP(&format, "format", "f", "json", "Output format (json, csv, txt)")
}

func runScan(cmd *cobra.Command, args []string) error {
	config := &scanner.Config{
		InputFile:  scanInputFile,
		OutputFile: scanOutputFile,
		Threads:    scanThreads,
		Timeout:    scanTimeout,
		ConfigFile: configFile,
		Format:     format,
		Verbose:    verbose,
	}

	s := scanner.New(config)

	if scanInputFile != "" {
		// Scan from input file
		return s.ScanFromFile(scanInputFile)
	} else {
		// Scan from stdin
		return s.ScanFromStdin()
	}
}