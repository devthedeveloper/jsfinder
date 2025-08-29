package discovery

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscovery_New(t *testing.T) {
	config := &Config{
		InputFile:    "test.txt",
		OutputFile:   "output.csv",
		WordlistFile: "wordlist.txt",
		Threads:      10,
		Timeout:      30,
		StatusFilter: "200,201,301,302",
		MaxRedirects: 5,
		UserAgent:    "jsfinder/1.0",
		Verbose:      false,
	}

	discovery := New(config)

	if discovery.config != config {
		t.Error("Expected config to be set")
	}

	if discovery.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if discovery.wordlist == nil {
		t.Error("Expected wordlist to be initialized")
	}

	if discovery.results == nil {
		t.Error("Expected results slice to be initialized")
	}

	// Check status filter parsing
	expectedStatuses := []int{200, 201, 301, 302}
	if len(discovery.statusFilter) != len(expectedStatuses) {
		t.Errorf("Expected %d status codes, got %d", len(expectedStatuses), len(discovery.statusFilter))
	}

	for _, expected := range expectedStatuses {
		if !discovery.statusFilter[expected] {
			t.Errorf("Expected status code %d to be in filter", expected)
		}
	}
}

func TestDiscovery_statusFilter(t *testing.T) {
	testCases := []struct {
		name     string
		filter   string
		expected []int
	}{
		{
			name:     "Single status",
			filter:   "200",
			expected: []int{200},
		},
		{
			name:     "Multiple statuses",
			filter:   "200,201,301,302,404",
			expected: []int{200, 201, 301, 302, 404},
		},
		{
			name:     "Empty filter",
			filter:   "",
			expected: []int{200},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				StatusFilter: tc.filter,
			}
			discovery := New(config)

			for _, expected := range tc.expected {
				if !discovery.statusFilter[expected] {
					t.Errorf("Expected status code %d to be in filter", expected)
				}
			}
		})
	}
}

func TestDiscovery_extractBaseURL(t *testing.T) {
	config := &Config{}
	discovery := New(config)

	testCases := []struct {
		name     string
		urlStr   string
		expected string
	}{
		{
			name:     "Full URL",
			urlStr:   "https://api.example.com/v1/users",
			expected: "https://api.example.com",
		},
		{
			name:     "URL with port",
			urlStr:   "http://localhost:8080/api/data",
			expected: "http://localhost:8080",
		},
		{
			name:     "HTTPS URL",
			urlStr:   "https://secure.example.com/admin/panel",
			expected: "https://secure.example.com",
		},
		{
			name:     "Invalid URL",
			urlStr:   "not-a-url",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := discovery.extractBaseURL(tc.urlStr)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestDiscovery_makeRequest(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"users": []}`))
		case "/admin":
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Forbidden"))
		case "/redirect":
			w.Header().Set("Location", "/api/users")
			w.WriteHeader(http.StatusFound)
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))
	defer server.Close()

	config := &Config{
		Threads:      1,
		Timeout:      10,
		StatusFilter: "200,403",
		MaxRedirects: 3,
		UserAgent:    "test-agent",
		Verbose:      false,
	}

	discovery := New(config)

	testCases := []struct {
		name           string
		url            string
		expectedStatus int
		shouldAdd      bool
	}{
		{
			name:           "Successful request",
			url:            server.URL + "/api/users",
			expectedStatus: 200,
			shouldAdd:      true,
		},
		{
			name:           "Forbidden request",
			url:            server.URL + "/admin",
			expectedStatus: 403,
			shouldAdd:      true,
		},
		{
			name:           "Not found request",
			url:            server.URL + "/nonexistent",
			expectedStatus: 404,
			shouldAdd:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear previous results
			discovery.results = []Endpoint{}

			discovery.makeRequest(tc.url, "GET", "test")

			if tc.shouldAdd {
				if len(discovery.results) == 0 {
					t.Error("Expected result to be added, but results are empty")
					return
				}

				result := discovery.results[0]
				if result.StatusCode != tc.expectedStatus {
					t.Errorf("Expected status code %d, got %d", tc.expectedStatus, result.StatusCode)
				}

				if result.URL != tc.url {
					t.Errorf("Expected URL %s, got %s", tc.url, result.URL)
				}
			} else {
				if len(discovery.results) > 0 {
					t.Error("Expected no results to be added, but results are not empty")
				}
			}
		})
	}
}

func TestDiscovery_discoverEndpoints(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"users": []}`))
		case "/api/admin":
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Forbidden"))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))
	defer server.Close()

	config := &Config{
		Threads:      2,
		Timeout:      10,
		StatusFilter: "200,403",
		MaxRedirects: 3,
		UserAgent:    "test-agent",
		Verbose:      false,
	}

	discovery := New(config)

	// Set up wordlist
	discovery.wordlist = []string{"users", "admin", "nonexistent"}

	// Add base URL to discovery
	discovery.baseURLs[server.URL] = true

	discovery.discoverEndpoints()

	// Should find 2 endpoints (users and admin)
	if len(discovery.results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(discovery.results))
	}

	// Check if expected endpoints are found
	foundUsers := false
	foundAdmin := false

	for _, result := range discovery.results {
		if strings.Contains(result.URL, "/api/users") && result.StatusCode == 200 {
			foundUsers = true
		}
		if strings.Contains(result.URL, "/api/admin") && result.StatusCode == 403 {
			foundAdmin = true
		}
	}

	if !foundUsers {
		t.Error("Expected to find /api/users endpoint")
	}

	if !foundAdmin {
		t.Error("Expected to find /api/admin endpoint")
	}
}

func TestDiscovery_loadWordlist(t *testing.T) {
	config := &Config{
		WordlistFile: "nonexistent.txt",
	}

	discovery := New(config)

	// Test with non-existent file (should use default wordlist)
	err := discovery.loadWordlist()
	if err != nil {
		t.Fatalf("Expected no error with non-existent wordlist, got: %v", err)
	}

	// Should have default wordlist
	if len(discovery.wordlist) == 0 {
		t.Error("Expected default wordlist to be loaded")
	}

	// Check if some common endpoints are in the default wordlist
	expectedEndpoints := []string{"api", "admin", "users", "login"}
	for _, expected := range expectedEndpoints {
		found := false
		for _, word := range discovery.wordlist {
			if word == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find '%s' in default wordlist", expected)
		}
	}
}

func TestDiscovery_DiscoverFromFile(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/test" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "success"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{
		Threads:      1,
		Timeout:      10,
		StatusFilter: "200",
		MaxRedirects: 3,
		UserAgent:    "test-agent",
		Verbose:      false,
	}

	discovery := New(config)

	// Set up wordlist
	discovery.wordlist = []string{"test", "nonexistent"}

	// Test with JS content containing base URL
	jsContent := fmt.Sprintf(`
		const API_BASE = '%s';
		fetch(API_BASE + '/api/data');
	`, server.URL)

	// Create a temporary JS file URL
	jsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsContent))
	}))
	defer jsServer.Close()

	// Test DiscoverFromFile with JS file URL
	err := discovery.DiscoverFromFile(jsServer.URL)
	if err != nil {
		t.Fatalf("Failed to discover from file: %v", err)
	}

	// Should find the test endpoint
	if len(discovery.results) == 0 {
		t.Error("Expected to find endpoints, but no results found")
	}
}

func TestDiscovery_DiscoverFromStdin(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/test" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "success"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{
		Threads:      1,
		Timeout:      10,
		StatusFilter: "200",
		MaxRedirects: 3,
		UserAgent:    "test-agent",
		Verbose:      false,
	}

	discovery := New(config)

	// Set up wordlist
	discovery.wordlist = []string{"test"}

	// Test DiscoverFromStdin
	err := discovery.DiscoverFromStdin()
	if err != nil {
		t.Fatalf("Failed to discover from stdin: %v", err)
	}

	// Note: This test doesn't provide actual stdin input,
	// so it should complete without errors but may not find results
}

// Benchmark tests
func BenchmarkDiscovery_extractBaseURLs(b *testing.B) {
	config := &Config{}
	discovery := New(config)

	testContent := `
		const API_BASE = 'https://api.example.com';
		const API_V2 = 'https://api-v2.example.com';
		fetch(API_BASE + '/users');
		axios.get(API_V2 + '/data');
		var endpoint = 'https://internal.example.com/admin';
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		discovery.extractBaseURLs(testContent)
	}
}

func BenchmarkDiscovery_parseStatusFilter(b *testing.B) {
	filter := "200,201,301,302,400,401,403,404,500,502,503"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test status filter creation
		testConfig := &Config{StatusFilter: filter}
		_ = New(testConfig)
	}
}