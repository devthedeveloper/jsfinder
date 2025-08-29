package crawler

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"jsfinder/pkg/utils"
)

// Config holds the configuration for the crawler
type Config struct {
	Domain       string
	OutputFile   string
	MaxDepth     int
	Threads      int
	Timeout      int
	IgnoreRobots bool
	Verbose      bool
}

// Crawler represents the web crawler
type Crawler struct {
	config        *Config
	client        *http.Client
	visited       map[string]bool
	visitedMux    sync.RWMutex
	jsFiles       map[string]bool
	jsFilesMux    sync.RWMutex
	output        *os.File
	logger        *utils.Logger
	timeoutMgr    *utils.TimeoutManager
	retryConfig   *utils.RetryConfig
}

// JSFile represents a discovered JavaScript file
type JSFile struct {
	URL    string `json:"url"`
	Source string `json:"source"`
	Size   int64  `json:"size"`
}

// New creates a new crawler instance
func New(config *Config) *Crawler {
	logger := utils.NewDefaultLogger()
	timeoutConfig := utils.CrawlerTimeoutConfig()
	timeoutMgr := utils.NewTimeoutManager(timeoutConfig, logger)
	retryConfig := utils.NetworkRetryConfig()
	
	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &Crawler{
		config:      config,
		client:      client,
		visited:     make(map[string]bool),
		jsFiles:     make(map[string]bool),
		logger:      logger,
		timeoutMgr:  timeoutMgr,
		retryConfig: retryConfig,
	}
}

// CrawlDomain crawls a single domain
func (c *Crawler) CrawlDomain(domain string) error {
	if c.config.Verbose {
		fmt.Printf("Starting crawl of domain: %s\n", domain)
	}

	if err := c.setupOutput(); err != nil {
		return fmt.Errorf("failed to setup output: %w", err)
	}
	defer c.closeOutput()

	return c.crawlURL(domain, 0)
}

// CrawlFromStdin crawls domains from stdin
func (c *Crawler) CrawlFromStdin() error {
	if err := c.setupOutput(); err != nil {
		return fmt.Errorf("failed to setup output: %w", err)
	}
	defer c.closeOutput()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" {
			if c.config.Verbose {
				fmt.Printf("Crawling domain: %s\n", domain)
			}
			if err := c.crawlURL(domain, 0); err != nil {
				fmt.Fprintf(os.Stderr, "Error crawling %s: %v\n", domain, err)
			}
		}
	}

	return scanner.Err()
}

func (c *Crawler) setupOutput() error {
	if c.config.OutputFile != "" {
		file, err := os.Create(c.config.OutputFile)
		if err != nil {
			return err
		}
		c.output = file
	} else {
		c.output = os.Stdout
	}
	return nil
}

func (c *Crawler) closeOutput() {
	if c.output != os.Stdout && c.output != nil {
		c.output.Close()
	}
}

func (c *Crawler) crawlURL(targetURL string, depth int) error {
	if depth > c.config.MaxDepth {
		return nil
	}

	c.visitedMux.Lock()
	if c.visited[targetURL] {
		c.visitedMux.Unlock()
		return nil
	}
	c.visited[targetURL] = true
	c.visitedMux.Unlock()

	// Create operation context with timeout
	opID := fmt.Sprintf("crawl-%s-%d", targetURL, depth)
	opCtx := c.timeoutMgr.CreateOperation(opID, 0) // Use default timeout
	defer c.timeoutMgr.CompleteOperation(opID)

	// Retry HTTP request with error handling
	var resp *http.Response
	var body []byte
	
	retryFn := func(ctx context.Context) error {
		// Send heartbeat
		c.timeoutMgr.SendHeartbeat(opID)
		
		req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			return utils.NewNetworkError(fmt.Sprintf("failed to create request for %s", targetURL), err)
		}
		
		resp, err = c.client.Do(req)
		if err != nil {
			return utils.NewNetworkError(fmt.Sprintf("failed to fetch %s", targetURL), err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode >= 400 {
			return utils.NewHTTPError(fmt.Sprintf("HTTP error for %s", targetURL), resp.StatusCode, nil)
		}
		
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewNetworkError(fmt.Sprintf("failed to read response body for %s", targetURL), err)
		}
		
		return nil
	}
	
	result := utils.Retry(opCtx.Ctx, c.retryConfig, retryFn, c.logger)
	if !result.Success {
		err := utils.WrapError(result.LastError, fmt.Sprintf("failed to crawl %s after %d attempts", targetURL, result.Attempts))
		utils.LogError(c.logger, err, map[string]interface{}{
			"url":      targetURL,
			"depth":    depth,
			"attempts": result.Attempts,
		})
		return err
	}

	// Extract JavaScript files from HTML
	c.extractJSFromHTML(string(body), targetURL)

	// Extract links for further crawling
	links := c.extractLinks(string(body), targetURL)

	// Crawl found links concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, c.config.Threads)

	for _, link := range links {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := c.crawlURL(url, depth+1); err != nil {
				utils.LogError(c.logger, err, map[string]interface{}{
					"url":   url,
					"depth": depth + 1,
				})
			}
		}(link)
	}

	wg.Wait()
	return nil
}

func (c *Crawler) extractJSFromHTML(htmlContent, baseURL string) {
	// Regex patterns for JavaScript files
	jsPatterns := []*regexp.Regexp{
		regexp.MustCompile(`<script[^>]+src=["']([^"']+\.js[^"']*)["']`),
		regexp.MustCompile(`<script[^>]+src=([^\s>]+\.js[^\s>]*)`),
	}

	for _, pattern := range jsPatterns {
		matches := pattern.FindAllStringSubmatch(htmlContent, -1)
		for _, match := range matches {
			if len(match) > 1 {
				jsURL := c.resolveURL(match[1], baseURL)
				c.addJSFile(jsURL)
			}
		}
	}
}

func (c *Crawler) extractLinks(htmlContent, baseURL string) []string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil
	}

	var links []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := c.resolveURL(attr.Val, baseURL)
					if c.isValidLink(link, baseURL) {
						links = append(links, link)
					}
					break
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			f(child)
		}
	}
	f(doc)

	return links
}

func (c *Crawler) resolveURL(href, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}

	ref, err := url.Parse(href)
	if err != nil {
		return href
	}

	return base.ResolveReference(ref).String()
}

func (c *Crawler) isValidLink(link, baseURL string) bool {
	parsedLink, err := url.Parse(link)
	if err != nil {
		return false
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	// Only crawl links from the same domain
	return parsedLink.Host == parsedBase.Host
}

func (c *Crawler) addJSFile(jsURL string) {
	c.jsFilesMux.Lock()
	defer c.jsFilesMux.Unlock()

	if !c.jsFiles[jsURL] {
		c.jsFiles[jsURL] = true
		if c.output != nil {
			fmt.Fprintln(c.output, jsURL)
		}
		if c.config.Verbose {
			fmt.Printf("Found JS file: %s\n", jsURL)
		}
	}
}