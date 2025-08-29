package cmd

import (
	"github.com/spf13/cobra"
	"jsfinder/pkg/discovery"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover endpoints using wordlists",
	Long: `Brute-force API endpoints using wordlists against discovered JavaScript files.
Analyzes JS content for potential endpoint patterns and tests them.`,
	Example: `  jsfinder discover --input jsfiles.txt --wordlist endpoints.txt --output endpoints.csv
  cat jsfiles.txt | jsfinder discover --wordlist common-endpoints.txt`,
	RunE: runDiscover,
}

var (
	discoverInputFile  string
	discoverOutputFile string
	wordlistFile       string
	discoverThreads    int
	discoverTimeout    int
	statusFilter       string
	maxRedirects       int
	userAgent          string
)

func init() {
	rootCmd.AddCommand(discoverCmd)

	discoverCmd.Flags().StringVarP(&discoverInputFile, "input", "i", "", "Input file containing JS file URLs")
	discoverCmd.Flags().StringVarP(&discoverOutputFile, "output", "o", "", "Output file for discovered endpoints")
	discoverCmd.Flags().StringVarP(&wordlistFile, "wordlist", "w", "", "Wordlist file for endpoint discovery")
	discoverCmd.Flags().IntVarP(&discoverThreads, "threads", "t", 20, "Number of concurrent threads")
	discoverCmd.Flags().IntVarP(&discoverTimeout, "timeout", "", 10, "Request timeout in seconds")
	discoverCmd.Flags().StringVarP(&statusFilter, "status", "s", "200,201,202,204,301,302,307,308,401,403", "HTTP status codes to report (comma-separated)")
	discoverCmd.Flags().IntVarP(&maxRedirects, "redirects", "r", 3, "Maximum number of redirects to follow")
	discoverCmd.Flags().StringVarP(&userAgent, "user-agent", "u", "jsfinder/1.0", "User-Agent header")

	// Make wordlist required
	discoverCmd.MarkFlagRequired("wordlist")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	config := &discovery.Config{
		InputFile:    discoverInputFile,
		OutputFile:   discoverOutputFile,
		WordlistFile: wordlistFile,
		Threads:      discoverThreads,
		Timeout:      discoverTimeout,
		StatusFilter: statusFilter,
		MaxRedirects: maxRedirects,
		UserAgent:    userAgent,
		Verbose:      verbose,
	}

	d := discovery.New(config)

	if discoverInputFile != "" {
		// Discover from input file
		return d.DiscoverFromFile(discoverInputFile)
	} else {
		// Discover from stdin
		return d.DiscoverFromStdin()
	}
}