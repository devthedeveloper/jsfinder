package crawler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCrawler_New(t *testing.T) {
	config := &Config{
		Domain:       "https://example.com",
		MaxDepth:     2,
		Threads:      5,
		Timeout:      30,
		IgnoreRobots: true,
		Verbose:      false,
	}

	crawler := New(config)

	if crawler.config != config {
		t.Error("Expected config to be set")
	}

	if crawler.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if crawler.visited == nil {
		t.Error("Expected visited map to be initialized")
	}

	if crawler.jsFiles == nil {
		t.Error("Expected jsFiles map to be initialized")
	}

	// Check client timeout
	expectedTimeout := time.Duration(config.Timeout) * time.Second
	if crawler.client.Timeout != expectedTimeout {
		t.Errorf("Expected client timeout %v, got %v", expectedTimeout, crawler.client.Timeout)
	}
}

func TestCrawler_extractJSFromHTML(t *testing.T) {
	testHTML := `
	<!DOCTYPE html>
	<html>
	<head>
		<script src="/js/app.js"></script>
		<script src="https://cdn.example.com/lib.js"></script>
		<script>var inline = "test";</script>
	</head>
	<body>
		<script src="./relative.js"></script>
		<script src="../parent.js"></script>
		<script type="text/javascript" src="/assets/main.js"></script>
		<script>
			// Inline JavaScript
			console.log("Hello World");
		</script>
	</body>
	</html>
	`

	config := &Config{
		Domain:   "https://example.com",
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	// Extract JS files using the actual method
	crawler.extractJSFromHTML(testHTML, "https://example.com/page")

	// Get the JS files from the crawler's map
	var jsFiles []string
	for jsFile := range crawler.jsFiles {
		jsFiles = append(jsFiles, jsFile)
	}

	// Should find external JS files
	expectedFiles := []string{
		"https://example.com/js/app.js",
		"https://cdn.example.com/lib.js",
		"https://example.com/relative.js",
		"https://example.com/parent.js",
		"https://example.com/assets/main.js",
	}

	if len(jsFiles) < len(expectedFiles) {
		t.Errorf("Expected at least %d JS files, got %d", len(expectedFiles), len(jsFiles))
	}

	// Check if expected files are found
	for _, expected := range expectedFiles {
		found := false
		for _, jsFile := range jsFiles {
			if jsFile == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find JS file: %s", expected)
		}
	}
}

func TestCrawler_extractLinks(t *testing.T) {
	testHTML := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Test Page</title>
	</head>
	<body>
		<a href="/page1">Page 1</a>
		<a href="https://example.com/page2">Page 2</a>
		<a href="./relative">Relative</a>
		<a href="../parent">Parent</a>
		<a href="https://external.com/page">External</a>
		<a href="mailto:test@example.com">Email</a>
		<a href="javascript:void(0)">JavaScript</a>
		<a href="#anchor">Anchor</a>
	</body>
	</html>
	`

	config := &Config{
		Domain:   "https://example.com",
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	links := crawler.extractLinks(testHTML, "https://example.com/test")

	// Should find internal links only
	expectedLinks := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/relative",
		"https://example.com/parent",
	}

	if len(links) < len(expectedLinks) {
		t.Errorf("Expected at least %d links, got %d", len(expectedLinks), len(links))
	}

	// Check if expected links are found
	for _, expected := range expectedLinks {
		found := false
		for _, link := range links {
			if link == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find link: %s", expected)
		}
	}

	// Should not find external links
	for _, link := range links {
		if strings.Contains(link, "external.com") {
			t.Errorf("Should not find external link: %s", link)
		}
		if strings.Contains(link, "mailto:") {
			t.Errorf("Should not find mailto link: %s", link)
		}
		if strings.Contains(link, "javascript:") {
			t.Errorf("Should not find javascript link: %s", link)
		}
		if strings.Contains(link, "#") {
			t.Errorf("Should not find anchor link: %s", link)
		}
	}
}

func TestCrawler_resolveURL(t *testing.T) {
	config := &Config{
		Domain:   "https://example.com",
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	testCases := []struct {
		name     string
		baseURL  string
		href     string
		expected string
	}{
		{
			name:     "Absolute URL",
			baseURL:  "https://example.com/page",
			href:     "https://example.com/other",
			expected: "https://example.com/other",
		},
		{
			name:     "Root relative",
			baseURL:  "https://example.com/dir/page",
			href:     "/assets/script.js",
			expected: "https://example.com/assets/script.js",
		},
		{
			name:     "Relative path",
			baseURL:  "https://example.com/dir/page",
			href:     "script.js",
			expected: "https://example.com/dir/script.js",
		},
		{
			name:     "Parent directory",
			baseURL:  "https://example.com/dir/subdir/page",
			href:     "../script.js",
			expected: "https://example.com/dir/script.js",
		},
		{
			name:     "Current directory",
			baseURL:  "https://example.com/dir/page",
			href:     "./script.js",
			expected: "https://example.com/dir/script.js",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := crawler.resolveURL(tc.baseURL, tc.href)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestCrawler_isValidLink(t *testing.T) {
	config := &Config{
		Domain:   "https://example.com",
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	testCases := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Valid internal URL",
			url:      "https://example.com/page",
			expected: true,
		},
		{
			name:     "Valid internal subdomain",
			url:      "https://api.example.com/endpoint",
			expected: true,
		},
		{
			name:     "External URL",
			url:      "https://external.com/page",
			expected: false,
		},
		{
			name:     "Mailto link",
			url:      "mailto:test@example.com",
			expected: false,
		},
		{
			name:     "JavaScript link",
			url:      "javascript:void(0)",
			expected: false,
		},
		{
			name:     "Anchor link",
			url:      "https://example.com/page#section",
			expected: false,
		},
		{
			name:     "File extension filter",
			url:      "https://example.com/image.jpg",
			expected: false,
		},
		{
			name:     "PDF file",
			url:      "https://example.com/document.pdf",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := crawler.isValidLink(tc.url, "https://example.com")
			if result != tc.expected {
				t.Errorf("Expected %v for %s, got %v", tc.expected, tc.url, result)
			}
		})
	}
}

func TestCrawler_crawlURL(t *testing.T) {
	// Create test server
	testHTML := `
	<!DOCTYPE html>
	<html>
	<head>
		<script src="/js/app.js"></script>
		<script src="https://cdn.example.com/lib.js"></script>
	</head>
	<body>
		<a href="/page1">Page 1</a>
		<a href="/page2">Page 2</a>
		<script>
			console.log("Inline JS");
		</script>
	</body>
	</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
	}))
	defer server.Close()

	config := &Config{
		Domain:   server.URL,
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	err := crawler.crawlURL(server.URL, 0)
	if err != nil {
		t.Fatalf("Failed to crawl URL: %v", err)
	}

	// Should have JS files in the map
	if len(crawler.jsFiles) == 0 {
		t.Error("Expected to find JS files, but none found")
	}
}

func TestCrawler_CrawlFromStdin(t *testing.T) {
	// Create test server
	testHTML := `
	<!DOCTYPE html>
	<html>
	<head>
		<script src="/js/test.js"></script>
	</head>
	<body>
		<h1>Test Page</h1>
	</body>
	</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
	}))
	defer server.Close()

	config := &Config{
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	// Test CrawlFromStdin method
	err := crawler.CrawlFromStdin()
	if err != nil {
		t.Fatalf("Failed to crawl from reader: %v", err)
	}

	// Should have found JS files
	if len(crawler.jsFiles) == 0 {
		t.Error("Expected to find JS files, but none found")
	}
}

func TestCrawler_jsFilesCollection(t *testing.T) {
	config := &Config{
		Domain:   "https://example.com",
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	// Add test JS files
	crawler.addJSFile("https://example.com/app.js")
	crawler.addJSFile("https://example.com/lib.js")

	// Check if JS files are collected
	if len(crawler.jsFiles) != 2 {
		t.Errorf("Expected 2 JS files, got %d", len(crawler.jsFiles))
	}

	if !crawler.jsFiles["https://example.com/app.js"] {
		t.Error("Expected app.js to be collected")
	}

	if !crawler.jsFiles["https://example.com/lib.js"] {
		t.Error("Expected lib.js to be collected")
	}
}

// Test edge cases
func TestCrawler_handleErrors(t *testing.T) {
	// Test 404 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	config := &Config{
		Domain:   server.URL,
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	err := crawler.crawlURL(server.URL, 0)
	if err == nil {
		t.Error("Expected error for 404 response, but got none")
	}
}

func TestCrawler_handleTimeout(t *testing.T) {
	// Test timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := &Config{
		Domain:   server.URL,
		MaxDepth: 1,
		Threads:  1,
		Timeout:  1, // 1 second timeout
		Verbose:  false,
	}

	crawler := New(config)

	err := crawler.crawlURL(server.URL, 0)
	if err == nil {
		t.Error("Expected timeout error, but got none")
	}
}

// Benchmark tests
func BenchmarkCrawler_extractJSFromHTML(b *testing.B) {
	testHTML := `
	<!DOCTYPE html>
	<html>
	<head>
		<script src="/js/app.js"></script>
		<script src="https://cdn.example.com/lib.js"></script>
		<script>var inline = "test";</script>
	</head>
	<body>
		<script src="./relative.js"></script>
		<script src="../parent.js"></script>
		<script type="text/javascript" src="/assets/main.js"></script>
	</body>
	</html>
	`

	config := &Config{
		Domain:   "https://example.com",
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crawler.extractJSFromHTML(testHTML, "https://example.com/page")
	}
}

func BenchmarkCrawler_extractLinks(b *testing.B) {
	testHTML := `
	<!DOCTYPE html>
	<html>
	<body>
		<a href="/page1">Page 1</a>
		<a href="https://example.com/page2">Page 2</a>
		<a href="./relative">Relative</a>
		<a href="../parent">Parent</a>
		<a href="https://external.com/page">External</a>
	</body>
	</html>
	`

	config := &Config{
		Domain:   "https://example.com",
		MaxDepth: 1,
		Threads:  1,
		Timeout:  10,
		Verbose:  false,
	}

	crawler := New(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crawler.extractLinks(testHTML, "https://example.com/test")
	}
}