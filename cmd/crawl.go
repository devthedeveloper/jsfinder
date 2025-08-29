package cmd

import (
	"github.com/spf13/cobra"
	"jsfinder/pkg/crawler"
)

var crawlCmd = &cobra.Command{
	Use:   "crawl",
	Short: "Crawl domains and extract JavaScript files",
	Long: `Crawl target domains to discover and extract JavaScript files.
Supports both single domain crawling and batch processing from stdin.`,
	Example: `  jsfinder crawl --domain https://example.com --output jsfiles.txt
  cat domains.txt | jsfinder crawl --output all-js.txt`,
	RunE: runCrawl,
}

var (
	domain     string
	outputFile string
	maxDepth   int
	threads    int
	timeout    int
	ignoreRobots bool
	verbose    bool
)

func init() {
	rootCmd.AddCommand(crawlCmd)

	crawlCmd.Flags().StringVarP(&domain, "domain", "d", "", "Target domain to crawl (e.g., https://example.com)")
	crawlCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for discovered JS files")
	crawlCmd.Flags().IntVarP(&maxDepth, "depth", "", 3, "Maximum crawl depth")
	crawlCmd.Flags().IntVarP(&threads, "threads", "t", 10, "Number of concurrent threads")
	crawlCmd.Flags().IntVarP(&timeout, "timeout", "", 30, "Request timeout in seconds")
	crawlCmd.Flags().BoolVarP(&ignoreRobots, "ignore-robots", "r", false, "Ignore robots.txt")
	crawlCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
}

func runCrawl(cmd *cobra.Command, args []string) error {
	config := &crawler.Config{
		Domain:       domain,
		OutputFile:   outputFile,
		MaxDepth:     maxDepth,
		Threads:      threads,
		Timeout:      timeout,
		IgnoreRobots: ignoreRobots,
		Verbose:      verbose,
	}

	c := crawler.New(config)

	if domain != "" {
		// Single domain crawling
		return c.CrawlDomain(domain)
	} else {
		// Batch processing from stdin
		return c.CrawlFromStdin()
	}
}