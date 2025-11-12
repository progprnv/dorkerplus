dorknewtool


@progprnv ➜ /workspaces/dorkerplus/test (main) $ cd /workspaces/dorkerplus/test && rm -rf results && ./dorkerplus -q "site: ext:pdf confidential" -pdf -max 5 2>&1 | head -50






# DorkPlus - Advanced Google Dorking & Screenshot Tool

DorkPlus is a powerful command-line tool written in Go that combines Google dorking capabilities with automated screenshot capture. It allows you to search for sensitive information across the web and automatically capture visual evidence.

## Features

🔍 **Google Dork Search**
- Execute complex Google search queries
- Support for all standard Google dork operators
- Multi-credential rotation with 5 pre-configured API keys

📸 **Automated Screenshots**
- Capture screenshots of discovered URLs
- Powered by Chromedp (Chrome DevTools Protocol)
- Automatic image size optimization (max 120KB)
- Organized storage with sanitized filenames

📝 **Smart Content Extraction**
- Automatic keyword matching from queries
- Extracts first line containing matching keywords
- Falls back to meaningful content if no match found
- Handles PDF and binary content gracefully

💾 **Results Management**
- Save all results to organized text file
- Screenshot organization by target
- Detailed output with URLs, titles, snippets, and matching lines

🎨 **Verbose Logging**
- Real-time progress display
- Color-coded output for easy reading
- File size tracking for screenshots

## Requirements

- Go 1.21 or higher
- Google Chrome or Chromium browser installed
- Valid Google Custom Search API credentials
- Unix-like environment (Linux, macOS) or Windows with appropriate tools

## Installation

### Prerequisites

1. **Install Go 1.21+**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install golang-go

   # macOS
   brew install go

   # Or download from https://golang.org/dl/
   ```

2. **Install Chrome/Chromium**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install chromium-browser

   # macOS
   brew install chromium
   ```

3. **Setup Project**
   ```bash
   cd /workspaces/dorkplus
   go mod download
   go build -o dorkplus
   ```

## Configuration

Edit `config.yaml` with your Google API credentials:

```yaml
google:
  - api_key: "YOUR_API_KEY"
    search_engine_id: "YOUR_SEARCH_ENGINE_ID"
  - api_key: "YOUR_API_KEY_2"
    search_engine_id: "YOUR_SEARCH_ENGINE_ID_2"
```

### Getting Google API Credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project
3. Enable Custom Search API
4. Create an API key
5. Create a Custom Search Engine at [cse.google.com/cse/](https://cse.google.com/cse/)
6. Get your Search Engine ID (cx parameter)
7. Add credentials to `config.yaml`

## Usage

### Basic Usage

```bash
./dorkplus -q "site:example.com ext:pdf confidential" -o results.txt -screenshots screenshots/
```

### Command-line Options

```
-q string
    Google dork query (required)

-o string
    Output file to save results (default: "dorks.txt")

-screenshots string
    Directory to save screenshots (optional)

-max int
    Maximum number of results (default: 10)

-timeout int
    Timeout in seconds for screenshot capture (default: 10)

-quiet
    Disable verbose output
```

### Examples

1. **Search and capture screenshots:**
   ```bash
   ./dorkplus -q "site:roche.com ext:pdf confidential" \
     -o results.txt \
     -screenshots roche_screenshots/
   ```

2. **High-volume search:**
   ```bash
   ./dorkplus -q "inurl:admin intitle:login" \
     -o admin_pages.txt \
     -screenshots admin_screenshots/ \
     -max 20
   ```

3. **Custom timeout:**
   ```bash
   ./dorkplus -q "site:github.com password" \
     -o github_results.txt \
     -screenshots github_shots/ \
     -timeout 15
   ```

4. **Quiet mode:**
   ```bash
   ./dorkplus -q "intitle:index.of" \
     -o dirs.txt \
     -quiet
   ```

## Output Format

### Results File

```
URL: https://example.com/page1.pdf
Title: Example Page Title
Matching Line: This page contains confidential information about...
Snippet: Example page snippet from search results...
────────────────────────────────────────────────────────────────────────────────

URL: https://example.com/page2.html
Title: Another Page
Matching Line: Confidential company data revealed in this document...
Snippet: More search result information...
────────────────────────────────────────────────────────────────────────────────
```

### Screenshot Directory

```
screenshots/
├── example_com_page1_pdf.png           (auto-optimized to ≤120KB)
├── example_com_page2_html.png
├── example_com_page3_pdf.png
└── ...
```

## How It Works

### 1. Search Phase
- Extracts keywords from your query
- Uses Google Custom Search API to find URLs
- Fetches content from each URL
- Matches keywords against fetched content

### 2. Screenshot Phase
- Uses Chrome DevTools Protocol (Chromedp)
- Captures full page screenshots
- Automatically optimizes file size to ≤120KB
- Saves with sanitized filenames

### 3. Results Phase
- Organizes all data
- Generates results file
- Links results to screenshots

## Supported Dork Operators

- `site:` - Search within specific domain
- `ext:` - Search for specific file types
- `inurl:` - Search in URL path
- `intitle:` - Search in page title
- `intext:` - Search in page content
- `filetype:` - Search for file types
- `cache:` - View cached version
- `link:` - Find pages linking to URL

## Performance Notes

- Screenshot capture adds significant time (default 10s timeout per URL)
- Maximum 10 results per search by default (adjustable with `-max`)
- Image optimization runs automatically
- Consider using `-quiet` flag for faster processing

## Troubleshooting

### Chrome/Chromium Not Found
```bash
# Install Chromium
sudo apt-get install chromium-browser

# Or specify Chrome path
export CHROME_BIN=/path/to/chrome
```

### API Rate Limiting
- Distribute searches across multiple API keys in config.yaml
- Add delays between searches if needed

### Screenshot Timeout
- Increase timeout with `-timeout` flag
- Ensure internet connection is stable

### Large Screenshot Files
- Tool automatically optimizes to ≤120KB
- If still too large, decrease resolution in screenshotter.go

## Building from Source

```bash
cd /workspaces/dorkplus
go mod download
go build -o dorkplus
```

### Cross-Platform Build

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 go build -o dorkplus-linux

# Build for macOS
GOOS=darwin GOARCH=amd64 go build -o dorkplus-darwin

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o dorkplus.exe
```

## Dependencies

- **google.golang.org/api** - Google APIs client
- **chromedp** - Chrome DevTools Protocol client
- **fatih/color** - Colored terminal output
- **gopkg.in/yaml.v2** - YAML configuration parsing

## Security & Legal

This tool is for:
- ✅ Authorized security testing
- ✅ Educational purposes
- ✅ Bug bounty programs
- ❌ Unauthorized access to systems
- ❌ Malicious activity

**Disclaimer:** Unauthorized access to computer systems is illegal. Always obtain proper authorization before using this tool for security research.

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Support

For issues, questions, or suggestions, please open an issue on the GitHub repository.

---

**Note:** Ensure you have the necessary permissions and legal authorization before using this tool for any security research or penetration testing activities.
