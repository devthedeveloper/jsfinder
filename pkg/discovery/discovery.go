package discovery

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds the configuration for endpoint discovery
type Config struct {
	InputFile    string
	OutputFile   string
	WordlistFile string
	Threads      int
	Timeout      int
	StatusFilter string
	MaxRedirects int
	UserAgent    string
	Verbose      bool
}

// Discovery represents the endpoint discovery engine
type Discovery struct {
	config        *Config
	client        *http.Client
	wordlist      []string
	statusFilter  map[int]bool
	results       []Endpoint
	mutex         sync.Mutex
	baseURLs      map[string]bool
	baseURLsMutex sync.RWMutex
}

// Endpoint represents a discovered endpoint
type Endpoint struct {
	URL            string `json:"url" csv:"url"`
	StatusCode     int    `json:"status_code" csv:"status_code"`
	ContentLength  int64  `json:"content_length" csv:"content_length"`
	ContentType    string `json:"content_type" csv:"content_type"`
	ResponseTime   int64  `json:"response_time_ms" csv:"response_time_ms"`
	Source         string `json:"source" csv:"source"`
	Method         string `json:"method" csv:"method"`
	RedirectChain  string `json:"redirect_chain,omitempty" csv:"redirect_chain"`
}

// New creates a new discovery instance
func New(config *Config) *Discovery {
	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	discovery := &Discovery{
		config:   config,
		client:   client,
		results:  make([]Endpoint, 0),
		baseURLs: make(map[string]bool),
	}

	discovery.parseStatusFilter()
	return discovery
}

// DiscoverFromFile discovers endpoints from JS files listed in input file
func (d *Discovery) DiscoverFromFile(inputFile string) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	return d.discoverFromReader(file)
}

// DiscoverFromStdin discovers endpoints from JS files from stdin
func (d *Discovery) DiscoverFromStdin() error {
	return d.discoverFromReader(os.Stdin)
}

func (d *Discovery) discoverFromReader(reader io.Reader) error {
	// Load wordlist
	if err := d.loadWordlist(); err != nil {
		return fmt.Errorf("failed to load wordlist: %w", err)
	}

	if d.config.Verbose {
		fmt.Printf("Loaded %d words from wordlist\n", len(d.wordlist))
	}

	// Extract base URLs from JS files
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		jsURL := strings.TrimSpace(scanner.Text())
		if jsURL != "" {
			if err := d.extractBaseURLs(jsURL); err != nil && d.config.Verbose {
				fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", jsURL, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if d.config.Verbose {
		fmt.Printf("Extracted %d unique base URLs\n", len(d.baseURLs))
	}

	// Discover endpoints
	if err := d.discoverEndpoints(); err != nil {
		return err
	}

	return d.outputResults()
}

func (d *Discovery) loadWordlist() error {
	file, err := os.Open(d.config.WordlistFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" && !strings.HasPrefix(word, "#") {
			d.wordlist = append(d.wordlist, word)
		}
	}

	return scanner.Err()
}

func (d *Discovery) extractBaseURLs(jsURL string) error {
	resp, err := d.client.Get(jsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, jsURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	content := string(body)

	// Extract potential API endpoints from JS content
	patterns := []*regexp.Regexp{
		// API endpoints in strings
		regexp.MustCompile(`["\'](https?://[^"'\s]*/(api|admin|v[0-9]+)/[^"'\s]*)["\']`),
		regexp.MustCompile(`["\'](/api/[^"'\s]*)["\']`),
		regexp.MustCompile(`["\'](/admin/[^"'\s]*)["\']`),
		regexp.MustCompile(`["\'](/v[0-9]+/[^"'\s]*)["\']`),
		// Base URLs
		regexp.MustCompile(`["\'](https?://[^"'/\s]+)["\']`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				baseURL := d.extractBaseURL(match[1])
				if baseURL != "" {
					d.baseURLsMutex.Lock()
					d.baseURLs[baseURL] = true
					d.baseURLsMutex.Unlock()
				}
			}
		}
	}

	// Also add the base URL of the JS file itself
	parsedURL, err := url.Parse(jsURL)
	if err == nil {
		baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
		d.baseURLsMutex.Lock()
		d.baseURLs[baseURL] = true
		d.baseURLsMutex.Unlock()
	}

	return nil
}

func (d *Discovery) extractBaseURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return ""
	}

	return fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
}

func (d *Discovery) discoverEndpoints() error {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, d.config.Threads)

	for baseURL := range d.baseURLs {
		for _, word := range d.wordlist {
			wg.Add(1)
			go func(base, endpoint string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				d.testEndpoint(base, endpoint)
			}(baseURL, word)
		}
	}

	wg.Wait()
	return nil
}

func (d *Discovery) testEndpoint(baseURL, endpoint string) {
	// Test different endpoint variations
	variations := []string{
		endpoint,
		"/" + endpoint,
		"/api/" + endpoint,
		"/api/v1/" + endpoint,
		"/api/v2/" + endpoint,
		"/admin/" + endpoint,
	}

	for _, variation := range variations {
		testURL := baseURL + variation
		d.makeRequest(testURL, "GET", baseURL)
	}
}

func (d *Discovery) makeRequest(testURL, method, source string) {
	start := time.Now()

	req, err := http.NewRequest(method, testURL, nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", d.config.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := d.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	responseTime := time.Since(start).Milliseconds()

	// Check if status code is in filter
	if !d.statusFilter[resp.StatusCode] {
		return
	}

	contentLength := resp.ContentLength
	if contentLength == -1 {
		// Try to read body to get actual length
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			contentLength = int64(len(body))
		}
	}

	contentType := resp.Header.Get("Content-Type")

	// Build redirect chain if any
	var redirectChain string
	if resp.Request.URL.String() != testURL {
		redirectChain = fmt.Sprintf("%s -> %s", testURL, resp.Request.URL.String())
	}

	endpoint := Endpoint{
		URL:           testURL,
		StatusCode:    resp.StatusCode,
		ContentLength: contentLength,
		ContentType:   contentType,
		ResponseTime:  responseTime,
		Source:        source,
		Method:        method,
		RedirectChain: redirectChain,
	}

	d.mutex.Lock()
	d.results = append(d.results, endpoint)
	d.mutex.Unlock()

	if d.config.Verbose {
		fmt.Printf("[%d] %s (%dms, %d bytes)\n", resp.StatusCode, testURL, responseTime, contentLength)
	}
}

func (d *Discovery) parseStatusFilter() {
	d.statusFilter = make(map[int]bool)
	statuses := strings.Split(d.config.StatusFilter, ",")
	for _, status := range statuses {
		code, err := strconv.Atoi(strings.TrimSpace(status))
		if err == nil {
			d.statusFilter[code] = true
		}
	}
}

func (d *Discovery) outputResults() error {
	if len(d.results) == 0 {
		if d.config.Verbose {
			fmt.Println("No endpoints discovered.")
		}
		return nil
	}

	var output io.Writer
	if d.config.OutputFile != "" {
		file, err := os.Create(d.config.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	} else {
		output = os.Stdout
	}

	// Default to CSV for discovery results
	if strings.HasSuffix(d.config.OutputFile, ".json") {
		return d.outputJSON(output)
	} else {
		return d.outputCSV(output)
	}
}

func (d *Discovery) outputJSON(output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(d.results)
}

func (d *Discovery) outputCSV(output io.Writer) error {
	writer := csv.NewWriter(output)
	defer writer.Flush()

	// Write header
	header := []string{"URL", "Status Code", "Content Length", "Content Type", "Response Time (ms)", "Source", "Method", "Redirect Chain"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, endpoint := range d.results {
		record := []string{
			endpoint.URL,
			fmt.Sprintf("%d", endpoint.StatusCode),
			fmt.Sprintf("%d", endpoint.ContentLength),
			endpoint.ContentType,
			fmt.Sprintf("%d", endpoint.ResponseTime),
			endpoint.Source,
			endpoint.Method,
			endpoint.RedirectChain,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}