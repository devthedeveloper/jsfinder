package scanner

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Config holds the configuration for the scanner
type Config struct {
	InputFile  string
	OutputFile string
	Threads    int
	Timeout    int
	ConfigFile string
	Format     string
	Verbose    bool
}

// Scanner represents the JavaScript file scanner
type Scanner struct {
	config   *Config
	client   *http.Client
	patterns map[string]*regexp.Regexp
	results  []Finding
	mutex    sync.Mutex
}

// Finding represents a discovered secret or sensitive information
type Finding struct {
	URL         string `json:"url" csv:"url"`
	Type        string `json:"type" csv:"type"`
	Pattern     string `json:"pattern" csv:"pattern"`
	Match       string `json:"match" csv:"match"`
	LineNumber  int    `json:"line_number" csv:"line_number"`
	Context     string `json:"context" csv:"context"`
	Confidence  string `json:"confidence" csv:"confidence"`
	Description string `json:"description" csv:"description"`
}

// New creates a new scanner instance
func New(config *Config) *Scanner {
	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	scanner := &Scanner{
		config:  config,
		client:  client,
		results: make([]Finding, 0),
	}

	scanner.initializePatterns()
	return scanner
}

// ScanFromFile scans JavaScript files listed in the input file
func (s *Scanner) ScanFromFile(inputFile string) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	return s.scanFromReader(file)
}

// ScanFromStdin scans JavaScript files from stdin
func (s *Scanner) ScanFromStdin() error {
	return s.scanFromReader(os.Stdin)
}

func (s *Scanner) scanFromReader(reader io.Reader) error {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, s.config.Threads)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		jsURL := strings.TrimSpace(scanner.Text())
		if jsURL != "" {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				if err := s.scanJSFile(url); err != nil && s.config.Verbose {
					fmt.Fprintf(os.Stderr, "Error scanning %s: %v\n", url, err)
				}
			}(jsURL)
		}
	}

	wg.Wait()

	if err := scanner.Err(); err != nil {
		return err
	}

	return s.outputResults()
}

func (s *Scanner) scanJSFile(jsURL string) error {
	if s.config.Verbose {
		fmt.Printf("Scanning: %s\n", jsURL)
	}

	resp, err := s.client.Get(jsURL)
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
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		s.scanLine(jsURL, line, lineNum+1)
	}

	return nil
}

func (s *Scanner) scanLine(jsURL, line string, lineNumber int) {
	for patternName, pattern := range s.patterns {
		matches := pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) > 0 {
				finding := Finding{
					URL:         jsURL,
					Type:        patternName,
					Pattern:     pattern.String(),
					Match:       match[0],
					LineNumber:  lineNumber,
					Context:     s.getContext(line, match[0]),
					Confidence:  s.getConfidence(patternName, match[0]),
					Description: s.getDescription(patternName),
				}

				s.mutex.Lock()
				s.results = append(s.results, finding)
				s.mutex.Unlock()

				if s.config.Verbose {
					fmt.Printf("Found %s: %s (line %d)\n", patternName, match[0], lineNumber)
				}
			}
		}
	}
}

func (s *Scanner) initializePatterns() {
	s.patterns = map[string]*regexp.Regexp{
		// AWS Keys
		"AWS_ACCESS_KEY":    regexp.MustCompile(`(?i)(aws_access_key_id|aws_access_key|aws_key_id)[\s]*[:=][\s]*["']?([A-Z0-9]{20})["']?`),
		"AWS_SECRET_KEY":    regexp.MustCompile(`(?i)(aws_secret_access_key|aws_secret_key)[\s]*[:=][\s]*["']?([A-Za-z0-9/+=]{40})["']?`),
		"AWS_SESSION_TOKEN": regexp.MustCompile(`(?i)(aws_session_token)[\s]*[:=][\s]*["']?([A-Za-z0-9/+=]{16,})["']?`),

		// Google Cloud Platform
		"GCP_API_KEY":     regexp.MustCompile(`(?i)(gcp_api_key|google_api_key)[\s]*[:=][\s]*["']?([A-Za-z0-9_-]{39})["']?`),
		"GCP_SERVICE_KEY": regexp.MustCompile(`(?i)"type"[\s]*:[\s]*"service_account"`),

		// Firebase
		"FIREBASE_API_KEY": regexp.MustCompile(`(?i)(firebase_api_key|firebase_key)[\s]*[:=][\s]*["']?([A-Za-z0-9_-]{39})["']?`),

		// GitHub
		"GITHUB_TOKEN": regexp.MustCompile(`(?i)(github_token|gh_token)[\s]*[:=][\s]*["']?(ghp_[A-Za-z0-9]{36}|gho_[A-Za-z0-9]{36}|ghu_[A-Za-z0-9]{36}|ghs_[A-Za-z0-9]{36}|ghr_[A-Za-z0-9]{36})["']?`),

		// JWT Tokens
		"JWT_TOKEN": regexp.MustCompile(`(?i)(jwt|token)[\s]*[:=][\s]*["']?(eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*)["']?`),

		// OAuth Tokens
		"OAUTH_TOKEN": regexp.MustCompile(`(?i)(oauth_token|access_token|bearer_token)[\s]*[:=][\s]*["']?([A-Za-z0-9_-]{20,})["']?`),

		// API Keys (Generic)
		"API_KEY": regexp.MustCompile(`(?i)(api_key|apikey|api-key)[\s]*[:=][\s]*["']?([A-Za-z0-9_-]{16,})["']?`),

		// Database URLs
		"DATABASE_URL": regexp.MustCompile(`(?i)(database_url|db_url)[\s]*[:=][\s]*["']?(mongodb://|mysql://|postgres://|redis://)[^"'\s]+["']?`),

		// Passwords
		"PASSWORD": regexp.MustCompile(`(?i)(password|passwd|pwd)[\s]*[:=][\s]*["']?([^"'\s]{8,})["']?`),

		// Secrets
		"SECRET": regexp.MustCompile(`(?i)(secret|secret_key)[\s]*[:=][\s]*["']?([A-Za-z0-9_-]{16,})["']?`),

		// Slack Tokens
		"SLACK_TOKEN": regexp.MustCompile(`(?i)(slack_token|slack_api_token)[\s]*[:=][\s]*["']?(xox[bpoa]-[0-9]{12}-[0-9]{12}-[0-9]{12}-[a-z0-9]{32})["']?`),

		// Stripe Keys
		"STRIPE_KEY": regexp.MustCompile(`(?i)(stripe_key|stripe_api_key)[\s]*[:=][\s]*["']?(sk_live_[A-Za-z0-9]{24}|pk_live_[A-Za-z0-9]{24})["']?`),

		// Twilio
		"TWILIO_SID": regexp.MustCompile(`(?i)(twilio_sid|account_sid)[\s]*[:=][\s]*["']?(AC[a-z0-9]{32})["']?`),

		// API Endpoints
		"API_ENDPOINT": regexp.MustCompile(`(?i)["\'](https?://[^"'\s]*/(api|admin|v[0-9]+)/[^"'\s]*)["\']`),

		// Internal Endpoints
		"INTERNAL_ENDPOINT": regexp.MustCompile(`(?i)["\'](/api/|/admin/|/internal/|/private/)[^"'\s]*["\']`),
	}
}

func (s *Scanner) getContext(line, match string) string {
	index := strings.Index(line, match)
	if index == -1 {
		return line
	}

	start := index - 20
	if start < 0 {
		start = 0
	}

	end := index + len(match) + 20
	if end > len(line) {
		end = len(line)
	}

	return line[start:end]
}

func (s *Scanner) getConfidence(patternType, match string) string {
	switch patternType {
	case "AWS_ACCESS_KEY", "AWS_SECRET_KEY", "GCP_SERVICE_KEY":
		return "HIGH"
	case "JWT_TOKEN", "GITHUB_TOKEN", "SLACK_TOKEN", "STRIPE_KEY":
		return "HIGH"
	case "API_KEY", "SECRET", "OAUTH_TOKEN":
		return "MEDIUM"
	case "PASSWORD", "DATABASE_URL":
		return "MEDIUM"
	case "API_ENDPOINT", "INTERNAL_ENDPOINT":
		return "LOW"
	default:
		return "LOW"
	}
}

func (s *Scanner) getDescription(patternType string) string {
	descriptions := map[string]string{
		"AWS_ACCESS_KEY":     "AWS Access Key ID",
		"AWS_SECRET_KEY":     "AWS Secret Access Key",
		"AWS_SESSION_TOKEN":  "AWS Session Token",
		"GCP_API_KEY":        "Google Cloud Platform API Key",
		"GCP_SERVICE_KEY":    "Google Cloud Service Account Key",
		"FIREBASE_API_KEY":   "Firebase API Key",
		"GITHUB_TOKEN":       "GitHub Personal Access Token",
		"JWT_TOKEN":          "JSON Web Token",
		"OAUTH_TOKEN":        "OAuth Access Token",
		"API_KEY":            "Generic API Key",
		"DATABASE_URL":       "Database Connection URL",
		"PASSWORD":           "Password or Credential",
		"SECRET":             "Secret Key",
		"SLACK_TOKEN":        "Slack API Token",
		"STRIPE_KEY":         "Stripe API Key",
		"TWILIO_SID":         "Twilio Account SID",
		"API_ENDPOINT":       "API Endpoint URL",
		"INTERNAL_ENDPOINT":  "Internal/Private Endpoint",
	}

	if desc, exists := descriptions[patternType]; exists {
		return desc
	}
	return "Unknown pattern type"
}

func (s *Scanner) outputResults() error {
	if len(s.results) == 0 {
		if s.config.Verbose {
			fmt.Println("No secrets or sensitive information found.")
		}
		return nil
	}

	var output io.Writer
	if s.config.OutputFile != "" {
		file, err := os.Create(s.config.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	} else {
		output = os.Stdout
	}

	switch strings.ToLower(s.config.Format) {
	case "json":
		return s.outputJSON(output)
	case "csv":
		return s.outputCSV(output)
	case "txt":
		return s.outputText(output)
	default:
		return s.outputJSON(output)
	}
}

func (s *Scanner) outputJSON(output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(s.results)
}

func (s *Scanner) outputCSV(output io.Writer) error {
	writer := csv.NewWriter(output)
	defer writer.Flush()

	// Write header
	header := []string{"URL", "Type", "Match", "Line Number", "Context", "Confidence", "Description"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, finding := range s.results {
		record := []string{
			finding.URL,
			finding.Type,
			finding.Match,
			fmt.Sprintf("%d", finding.LineNumber),
			finding.Context,
			finding.Confidence,
			finding.Description,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scanner) outputText(output io.Writer) error {
	for _, finding := range s.results {
		fmt.Fprintf(output, "[%s] %s\n", finding.Confidence, finding.Type)
		fmt.Fprintf(output, "  URL: %s\n", finding.URL)
		fmt.Fprintf(output, "  Match: %s\n", finding.Match)
		fmt.Fprintf(output, "  Line: %d\n", finding.LineNumber)
		fmt.Fprintf(output, "  Context: %s\n", finding.Context)
		fmt.Fprintf(output, "  Description: %s\n", finding.Description)
		fmt.Fprintf(output, "\n")
	}
	return nil
}