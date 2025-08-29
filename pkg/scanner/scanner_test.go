package scanner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScanner_initializePatterns(t *testing.T) {
	config := &Config{
		Threads: 1,
		Timeout: 10,
		Format:  "json",
		Verbose: false,
	}

	scanner := New(config)

	// Test that patterns are initialized
	if len(scanner.patterns) == 0 {
		t.Error("Expected patterns to be initialized, got empty map")
	}

	// Test specific patterns exist
	expectedPatterns := []string{
		"AWS_ACCESS_KEY",
		"AWS_SECRET_KEY",
		"JWT_TOKEN",
		"API_KEY",
		"GITHUB_TOKEN",
	}

	for _, pattern := range expectedPatterns {
		if _, exists := scanner.patterns[pattern]; !exists {
			t.Errorf("Expected pattern %s to exist", pattern)
		}
	}
}

func TestScanner_scanLine(t *testing.T) {
	config := &Config{
		Threads: 1,
		Timeout: 10,
		Format:  "json",
		Verbose: false,
	}

	scanner := New(config)

	testCases := []struct {
		name         string
		line         string
		expectedType string
		shouldFind   bool
	}{
		{
			name:         "AWS Access Key",
			line:         `aws_access_key_id = "AKIAIOSFODNN7EXAMPLE"`,
			expectedType: "AWS_ACCESS_KEY",
			shouldFind:   true,
		},
		{
			name:         "AWS Secret Key",
			line:         `aws_secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`,
			expectedType: "AWS_SECRET_KEY",
			shouldFind:   true,
		},
		{
			name:         "JWT Token",
			line:         `token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"`,
			expectedType: "JWT_TOKEN",
			shouldFind:   true,
		},
		{
			name:         "API Key",
			line:         `api_key: "sk-1234567890abcdef1234567890abcdef"`,
			expectedType: "API_KEY",
			shouldFind:   true,
		},
		{
			name:         "GitHub Token",
			line:         `github_token = "ghp_1234567890abcdef1234567890abcdef123456"`,
			expectedType: "GITHUB_TOKEN",
			shouldFind:   true,
		},
		{
			name:         "API Endpoint",
			line:         `url: "https://api.example.com/v1/users"`,
			expectedType: "API_ENDPOINT",
			shouldFind:   true,
		},
		{
			name:         "No secrets",
			line:         `var x = "hello world";`,
			expectedType: "",
			shouldFind:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear previous results
			scanner.results = []Finding{}

			scanner.scanLine("https://example.com/test.js", tc.line, 1)

			if tc.shouldFind {
				if len(scanner.results) == 0 {
					t.Errorf("Expected to find %s, but no results found", tc.expectedType)
					return
				}

				found := false
				for _, result := range scanner.results {
					if result.Type == tc.expectedType {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("Expected to find %s, but found %v", tc.expectedType, scanner.results)
				}
			} else {
				if len(scanner.results) > 0 {
					t.Errorf("Expected no results, but found %v", scanner.results)
				}
			}
		})
	}
}

func TestScanner_getConfidence(t *testing.T) {
	config := &Config{}
	scanner := New(config)

	testCases := []struct {
		patternType        string
		expectedConfidence string
	}{
		{"AWS_ACCESS_KEY", "HIGH"},
		{"AWS_SECRET_KEY", "HIGH"},
		{"JWT_TOKEN", "HIGH"},
		{"API_KEY", "MEDIUM"},
		{"SECRET", "MEDIUM"},
		{"API_ENDPOINT", "LOW"},
		{"UNKNOWN_PATTERN", "LOW"},
	}

	for _, tc := range testCases {
		t.Run(tc.patternType, func(t *testing.T) {
			confidence := scanner.getConfidence(tc.patternType, "dummy_match")
			if confidence != tc.expectedConfidence {
				t.Errorf("Expected confidence %s for %s, got %s", tc.expectedConfidence, tc.patternType, confidence)
			}
		})
	}
}

func TestScanner_getContext(t *testing.T) {
	config := &Config{}
	scanner := New(config)

	testCases := []struct {
		name     string
		line     string
		match    string
		expected string
	}{
		{
			name:     "Short line",
			line:     "api_key = 'secret'",
			match:    "secret",
			expected: "api_key = 'secret'",
		},
		{
			name:     "Long line with context",
			line:     "const config = { database: { host: 'localhost', password: 'supersecret123', port: 5432 } };",
			match:    "supersecret123",
			expected: "alhost', password: 'supersecret123', port: 5432 } };",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			context := scanner.getContext(tc.line, tc.match)
			if context != tc.expected {
				t.Errorf("Expected context '%s', got '%s'", tc.expected, context)
			}
		})
	}
}

func TestScanner_outputJSON(t *testing.T) {
	config := &Config{
		Format: "json",
	}
	scanner := New(config)

	// Add test results
	scanner.results = []Finding{
		{
			URL:         "https://example.com/test.js",
			Type:        "API_KEY",
			Match:       "sk-1234567890abcdef",
			LineNumber:  10,
			Context:     "api_key = 'sk-1234567890abcdef'",
			Confidence:  "MEDIUM",
			Description: "Generic API Key",
		},
	}

	var buf bytes.Buffer
	err := scanner.outputJSON(&buf)
	if err != nil {
		t.Fatalf("Failed to output JSON: %v", err)
	}

	// Parse the JSON to verify it's valid
	var results []Finding
	err = json.Unmarshal(buf.Bytes(), &results)
	if err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].Type != "API_KEY" {
		t.Errorf("Expected type API_KEY, got %s", results[0].Type)
	}
}

func TestScanner_scanJSFile(t *testing.T) {
	// Create a test server
	testJS := `
		const config = {
			aws_access_key_id: "AKIAIOSFODNN7EXAMPLE",
			aws_secret_access_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			api_key: "sk-1234567890abcdef1234567890abcdef",
			jwt_token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.test"
		};
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJS))
	}))
	defer server.Close()

	config := &Config{
		Threads: 1,
		Timeout: 10,
		Format:  "json",
		Verbose: false,
	}

	scanner := New(config)

	err := scanner.scanJSFile(server.URL)
	if err != nil {
		t.Fatalf("Failed to scan JS file: %v", err)
	}

	// Should find multiple secrets
	if len(scanner.results) < 3 {
		t.Errorf("Expected at least 3 findings, got %d", len(scanner.results))
	}

	// Check for specific findings
	foundTypes := make(map[string]bool)
	for _, result := range scanner.results {
		foundTypes[result.Type] = true
	}

	expectedTypes := []string{"AWS_ACCESS_KEY", "AWS_SECRET_KEY", "API_KEY", "JWT_TOKEN"}
	for _, expectedType := range expectedTypes {
		if !foundTypes[expectedType] {
			t.Errorf("Expected to find %s, but it was not found", expectedType)
		}
	}
}

func TestScanner_scanFromReader(t *testing.T) {
	// Create a test server
	testJS := `
		const secrets = {
			api_key: "test-api-key-123456789",
			secret: "super-secret-value"
		};
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testJS))
	}))
	defer server.Close()

	config := &Config{
		Threads: 1,
		Timeout: 10,
		Format:  "json",
		Verbose: false,
	}

	scanner := New(config)

	// Create input with JS file URL
	input := strings.NewReader(server.URL)

	err := scanner.scanFromReader(input)
	if err != nil {
		t.Fatalf("Failed to scan from reader: %v", err)
	}

	// Should find secrets
	if len(scanner.results) == 0 {
		t.Error("Expected to find secrets, but no results found")
	}
}

func TestScanner_getDescription(t *testing.T) {
	config := &Config{}
	scanner := New(config)

	testCases := []struct {
		patternType string
		expected    string
	}{
		{"AWS_ACCESS_KEY", "AWS Access Key ID"},
		{"JWT_TOKEN", "JSON Web Token"},
		{"API_KEY", "Generic API Key"},
		{"UNKNOWN_PATTERN", "Unknown pattern type"},
	}

	for _, tc := range testCases {
		t.Run(tc.patternType, func(t *testing.T) {
			description := scanner.getDescription(tc.patternType)
			if description != tc.expected {
				t.Errorf("Expected description '%s' for %s, got '%s'", tc.expected, tc.patternType, description)
			}
		})
	}
}

// Benchmark tests
func BenchmarkScanner_scanLine(b *testing.B) {
	config := &Config{
		Threads: 1,
		Timeout: 10,
		Format:  "json",
		Verbose: false,
	}

	scanner := New(config)
	testLine := `const config = { aws_access_key_id: "AKIAIOSFODNN7EXAMPLE", api_key: "sk-1234567890abcdef", jwt: "eyJhbGciOiJIUzI1NiJ9.test.sig" };`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.results = []Finding{} // Clear results
		scanner.scanLine("https://example.com/test.js", testLine, 1)
	}
}

func BenchmarkScanner_initializePatterns(b *testing.B) {
	config := &Config{
		Threads: 1,
		Timeout: 10,
		Format:  "json",
		Verbose: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := New(config)
		_ = scanner.patterns
	}
}