# JSFinder

A powerful and comprehensive JavaScript file discovery and security analysis tool designed for penetration testers, bug bounty hunters, and security researchers. JSFinder combines web crawling, endpoint discovery, and secret scanning capabilities to help identify potential security vulnerabilities in web applications.

## Features

### 🕷️ Web Crawling
- **Domain Crawling**: Recursively crawl websites to discover JavaScript files
- **Concurrent Processing**: Multi-threaded crawling for improved performance
- **Depth Control**: Configurable crawling depth to manage scope
- **Robots.txt Support**: Option to respect or ignore robots.txt directives
- **Timeout Management**: Built-in timeout and retry mechanisms
- **Error Handling**: Robust error handling with structured logging

### 🔍 Secret Scanning
- **Pattern Detection**: Comprehensive regex patterns for detecting:
  - API Keys (AWS, Google Cloud, Firebase, GitHub, etc.)
  - Authentication Tokens (JWT, OAuth, Bearer tokens)
  - Database URLs and connection strings
  - Private keys and certificates
  - Internal endpoints and URLs
- **Confidence Scoring**: Intelligent confidence levels for detected secrets
- **Context Extraction**: Provides surrounding code context for findings
- **Multiple Output Formats**: JSON and CSV output support

### 🎯 Endpoint Discovery
- **Wordlist-based Discovery**: Brute force endpoint discovery using custom wordlists
- **JavaScript Analysis**: Extract base URLs and endpoints from JavaScript files
- **Status Code Filtering**: Filter results by HTTP status codes
- **Concurrent Requests**: Multi-threaded endpoint testing
- **Rate Limiting**: Built-in rate limiting and retry logic

### 🛠️ Advanced Features
- **Structured Logging**: Comprehensive logging with multiple levels
- **Configuration Management**: YAML-based configuration system
- **Retry Logic**: Exponential backoff with jitter for failed requests
- **Timeout Management**: Global and per-operation timeout controls
- **Error Recovery**: Graceful error handling and recovery mechanisms
- **Performance Monitoring**: Built-in performance metrics and statistics

## Installation

### Prerequisites
- Go 1.25.0 or higher
- Git

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/jsfinder.git
cd jsfinder

# Install dependencies
go mod download

# Build the application
make build

# Or install directly
go install
```

### Using Go Install

```bash
go install github.com/yourusername/jsfinder@latest
```

### Binary Releases

Download pre-compiled binaries from the [releases page](https://github.com/yourusername/jsfinder/releases).

## Quick Start

### Basic Web Crawling

```bash
# Crawl a domain and save JavaScript files
jsfinder crawl -d example.com -o js_files.txt

# Crawl with custom depth and threads
jsfinder crawl -d example.com -o js_files.txt --depth 3 --threads 10

# Crawl from stdin (useful for piping URLs)
echo "https://example.com" | jsfinder crawl --stdin -o js_files.txt
```

### Secret Scanning

```bash
# Scan JavaScript files for secrets
jsfinder scan -f js_files.txt -o secrets.json

# Scan with custom patterns
jsfinder scan -f js_files.txt -p custom_patterns.yaml -o secrets.json

# Scan from stdin
cat js_files.txt | jsfinder scan --stdin -o secrets.csv --format csv
```

### Endpoint Discovery

```bash
# Discover endpoints using wordlist
jsfinder discover -f js_files.txt -w config/endpoints.txt -o endpoints.json

# Filter by status codes
jsfinder discover -f js_files.txt -w wordlist.txt --status "200,201,301,302" -o endpoints.json

# Custom threads and timeout
jsfinder discover -f js_files.txt -w wordlist.txt --threads 20 --timeout 10 -o endpoints.json
```

## Usage Examples

### Complete Security Assessment Workflow

```bash
# Step 1: Crawl the target domain
jsfinder crawl -d target.com -o js_files.txt --depth 5 --threads 15 --verbose

# Step 2: Scan for secrets in discovered JavaScript files
jsfinder scan -f js_files.txt -o secrets.json --format json

# Step 3: Discover additional endpoints
jsfinder discover -f js_files.txt -w config/endpoints.txt -o endpoints.json --threads 10

# Step 4: Review findings
cat secrets.json | jq '.[] | select(.confidence > 0.8)'
cat endpoints.json | jq '.[] | select(.status_code == 200)'
```

### Advanced Configuration

```bash
# Use custom configuration file
jsfinder scan -f js_files.txt -c custom_config.yaml -o results.json

# Combine multiple operations
jsfinder crawl -d example.com --stdout | jsfinder scan --stdin --stdout | jsfinder discover --stdin -w endpoints.txt
```

### Integration with Other Tools

```bash
# Integration with subfinder and httpx
subfinder -d target.com | httpx -silent | while read url; do
    echo "$url" | jsfinder crawl --stdin --stdout
done | jsfinder scan --stdin -o all_secrets.json

# Integration with waybackurls
waybackurls target.com | grep -E '\.js$' | jsfinder scan --stdin -o wayback_secrets.json
```

## Configuration

### Configuration File Structure

JSFinder uses YAML configuration files. The default configuration is located at `config/patterns.yaml`:

```yaml
patterns:
  secrets:
    aws_access_key:
      pattern: "AKIA[0-9A-Z]{16}"
      description: "AWS Access Key ID"
      confidence: 0.9
    
    github_token:
      pattern: "ghp_[a-zA-Z0-9]{36}"
      description: "GitHub Personal Access Token"
      confidence: 0.95

crawler:
  default_timeout: 30
  max_depth: 5
  default_threads: 10
  user_agent: "JSFinder/1.0"

scanner:
  max_file_size: 10485760  # 10MB
  context_lines: 3
  min_confidence: 0.5

discovery:
  default_threads: 5
  timeout: 15
  max_redirects: 3
  status_filter: "200,201,301,302,403"
```

### Custom Patterns

Create custom pattern files for specific use cases:

```yaml
patterns:
  custom:
    internal_api:
      pattern: "/api/v[0-9]+/internal/"
      description: "Internal API endpoint"
      confidence: 0.8
    
    debug_info:
      pattern: "console\.(log|debug|error)\(['\"].*['\"]\)"
      description: "Debug console output"
      confidence: 0.6
```

## Command Reference

### Global Flags

- `--config, -c`: Configuration file path
- `--verbose, -v`: Enable verbose output
- `--help, -h`: Show help information

### Crawl Command

```bash
jsfinder crawl [flags]
```

**Flags:**
- `--domain, -d`: Target domain to crawl
- `--output, -o`: Output file for discovered JavaScript files
- `--depth`: Maximum crawling depth (default: 3)
- `--threads`: Number of concurrent threads (default: 10)
- `--timeout`: Request timeout in seconds (default: 30)
- `--ignore-robots`: Ignore robots.txt directives
- `--stdin`: Read URLs from stdin
- `--stdout`: Output results to stdout

### Scan Command

```bash
jsfinder scan [flags]
```

**Flags:**
- `--file, -f`: Input file containing JavaScript files/URLs
- `--output, -o`: Output file for scan results
- `--patterns, -p`: Custom patterns file
- `--format`: Output format (json, csv) (default: json)
- `--min-confidence`: Minimum confidence threshold (default: 0.5)
- `--stdin`: Read input from stdin
- `--stdout`: Output results to stdout

### Discover Command

```bash
jsfinder discover [flags]
```

**Flags:**
- `--file, -f`: Input file containing JavaScript files/URLs
- `--wordlist, -w`: Wordlist file for endpoint discovery
- `--output, -o`: Output file for discovered endpoints
- `--threads`: Number of concurrent threads (default: 5)
- `--timeout`: Request timeout in seconds (default: 15)
- `--status`: Comma-separated list of status codes to include
- `--stdin`: Read input from stdin
- `--stdout`: Output results to stdout

## Output Formats

### JSON Output (Default)

```json
{
  "url": "https://example.com/app.js",
  "type": "aws_access_key",
  "description": "AWS Access Key ID",
  "match": "AKIAIOSFODNN7EXAMPLE",
  "confidence": 0.9,
  "line_number": 42,
  "context": {
    "before": ["const config = {", "  region: 'us-east-1',"],
    "match": "  accessKeyId: 'AKIAIOSFODNN7EXAMPLE',",
    "after": ["  secretAccessKey: process.env.SECRET,", "}"]
  },
  "timestamp": "2024-01-15T10:30:45Z"
}
```

### CSV Output

```csv
url,type,description,match,confidence,line_number,timestamp
https://example.com/app.js,aws_access_key,AWS Access Key ID,AKIAIOSFODNN7EXAMPLE,0.9,42,2024-01-15T10:30:45Z
```

## Performance Tuning

### Optimizing Crawling Performance

```bash
# Increase threads for faster crawling
jsfinder crawl -d example.com --threads 20 --timeout 10

# Limit depth for large sites
jsfinder crawl -d example.com --depth 2 --threads 15
```

### Memory Management

```bash
# For large-scale scanning, process in batches
split -l 1000 large_js_list.txt batch_
for batch in batch_*; do
    jsfinder scan -f "$batch" -o "results_${batch}.json"
done
```

### Rate Limiting

JSFinder includes built-in rate limiting and retry mechanisms:

- Exponential backoff with jitter
- Configurable retry attempts
- Timeout management
- Graceful error handling

## Troubleshooting

### Common Issues

**1. Connection Timeouts**
```bash
# Increase timeout values
jsfinder crawl -d example.com --timeout 60
```

**2. Rate Limiting**
```bash
# Reduce concurrent threads
jsfinder discover -f js_files.txt --threads 3
```

**3. Memory Issues**
```bash
# Process files in smaller batches
# Use streaming mode with stdin/stdout
cat large_file.txt | jsfinder scan --stdin --stdout > results.json
```

### Debug Mode

```bash
# Enable verbose logging
jsfinder crawl -d example.com --verbose

# Check configuration
jsfinder --help
```

## Development

### Building from Source

```bash
# Clone and build
git clone https://github.com/yourusername/jsfinder.git
cd jsfinder
make build

# Run tests
make test

# Run with coverage
make test-coverage

# Lint code
make lint
```

### Project Structure

```
jsfinder/
├── cmd/                 # CLI commands
│   ├── crawl.go
│   ├── discover.go
│   ├── root.go
│   └── scan.go
├── pkg/                 # Core packages
│   ├── crawler/         # Web crawling logic
│   ├── discovery/       # Endpoint discovery
│   ├── scanner/         # Secret scanning
│   └── utils/           # Utilities (logging, errors, retry)
├── config/              # Configuration files
│   ├── patterns.yaml    # Default patterns
│   └── endpoints.txt    # Default wordlist
├── Makefile            # Build automation
├── go.mod              # Go modules
└── README.md           # This file
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

### Adding Custom Patterns

To add new detection patterns:

1. Edit `config/patterns.yaml`
2. Add your pattern following the existing format
3. Test with sample data
4. Submit a pull request

## Security Considerations

### Responsible Disclosure

- Only test on systems you own or have explicit permission to test
- Follow responsible disclosure practices for any vulnerabilities found
- Respect rate limits and avoid overwhelming target systems

### Data Handling

- JSFinder may discover sensitive information
- Ensure secure storage and handling of scan results
- Consider encrypting output files containing sensitive data
- Follow your organization's data handling policies

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Thanks to the security research community for pattern contributions
- Inspired by various OSINT and security testing tools
- Built with Go and various open-source libraries

## Support

- 📧 Email: support@jsfinder.dev
- 🐛 Issues: [GitHub Issues](https://github.com/yourusername/jsfinder/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/yourusername/jsfinder/discussions)
- 📖 Documentation: [Wiki](https://github.com/yourusername/jsfinder/wiki)

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history and updates.

---

**⚠️ Disclaimer**: This tool is intended for authorized security testing and research purposes only. Users are responsible for ensuring they have proper authorization before testing any systems.