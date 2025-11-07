package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	// Define flags
	query := flag.String("q", "", "Google dork query (required)")
	output := flag.String("o", "dorks.txt", "Output file to save results")
	screenshotDir := flag.String("screenshots", "", "Directory to save screenshots")
	maxResults := flag.Int("max", 10, "Maximum number of results")
	quiet := flag.Bool("quiet", false, "Disable verbose output")
	timeout := flag.Int("timeout", 10, "Timeout in seconds for screenshot capture")
	pdfMode := flag.Bool("pdf", false, "PDF mode: download and capture PDF screenshots only")

	flag.Parse()

	// Validate query
	if *query == "" {
		fmt.Fprintf(os.Stderr, "[ERROR] Query is required. Use -q flag\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	verbose := !*quiet

	if verbose {
		fmt.Println("============================================================")
		fmt.Println("DorkPlus - Google Dorking + Screenshot Tool")
		fmt.Println("============================================================")
	}

	// Load config
	config, err := LoadConfig("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("[+] Loaded %d API credentials\n", len(config.Google))
	}

	// Extract keywords from query
	keywords := extractKeywords(*query)
	if verbose {
		fmt.Printf("[*] Keywords to match: %v\n", keywords)
	}

	// Create dorker
	dorker := NewGoogleDorker(config, verbose)

	// Perform search
	ctx := context.Background()
	results, err := dorker.Search(ctx, *query, *maxResults)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Search failed: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "[ERROR] No results found\n")
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("[+] Found %d results\n", len(results))
	}

	// Extract matching lines and add keywords to each result
	for i := range results {
		results[i].MatchingLine = extractMatchingLine(results[i].Content, keywords)
		results[i].Keywords = keywords
		if verbose {
			fmt.Printf("      Matching line: %s...\n", truncate(results[i].MatchingLine, 80))
		}
	}

	// Create screenshot directory if needed
	var screenshotPath string
	if *screenshotDir != "" {
		screenshotPath = *screenshotDir
		if err := os.MkdirAll(screenshotPath, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to create screenshot directory: %v\n", err)
			os.Exit(1)
		}
		if verbose {
			fmt.Printf("[+] Screenshot directory created: %s\n", screenshotPath)
		}
	}

	// Capture screenshots if requested
	if screenshotPath != "" {
		if *pdfMode {
			fmt.Println("[*] PDF Mode: Capturing PDF screenshots only...")
			captureScreenshotsWithPDF(ctx, results, screenshotPath, *timeout, verbose)
		} else {
			fmt.Println("[*] Capturing screenshots...")
			captureScreenshots(ctx, results, screenshotPath, *timeout, verbose)
		}
	}

	// Save results to file
	if err := saveResults(results, *output); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to save results: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("[+] Results saved to: %s\n", *output)
		fmt.Printf("[+] Total results: %d\n", len(results))
		fmt.Println("============================================================\n")
	}
}

// extractKeywords extracts keywords from a dork query
func extractKeywords(query string) []string {
	operators := []string{"site:", "ext:", "inurl:", "intitle:", "intext:", "filetype:", "cache:", "link:"}

	parts := strings.Fields(query)
	var keywords []string

	for _, part := range parts {
		isOperator := false
		for _, op := range operators {
			if strings.HasPrefix(strings.ToLower(part), op) {
				isOperator = true
				break
			}
		}

		if !isOperator {
			// Remove quotes
			cleanedPart := strings.Trim(part, "\"'")
			if cleanedPart != "" {
				keywords = append(keywords, cleanedPart)
			}
		}
	}

	if len(keywords) == 0 {
		return []string{""}
	}
	return keywords
}

// extractMatchingLine finds the first line that matches keywords
func extractMatchingLine(content string, keywords []string) string {
	lines := strings.Split(content, "\n")

	// Remove non-printable characters
	re := regexp.MustCompile(`[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F-\xFF]+`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 5 {
			continue
		}

		// Clean line
		line = re.ReplaceAllString(line, " ")
		line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")

		// Check if any keyword matches
		for _, keyword := range keywords {
			if strings.Contains(strings.ToLower(line), strings.ToLower(keyword)) {
				if len(line) > 200 {
					return line[:200]
				}
				return line
			}
		}
	}

	// Fallback: return first meaningful line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 && !strings.HasPrefix(line, "%PDF") {
			line = re.ReplaceAllString(line, " ")
			line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")
			if len(line) > 200 {
				return line[:200]
			}
			return line
		}
	}

	return "No content extracted"
}

// captureScreenshotsWithPDF downloads PDFs and captures real screenshots
func captureScreenshotsWithPDF(ctx context.Context, results []Result, outputDir string, timeout int, verbose bool) {
	screenshotter := NewScreenshotter(timeout, verbose)
	defer screenshotter.Close()

	for i, result := range results {
		// Generate filename from URL
		filename := sanitizeFilename(result.URL)
		filePath := filepath.Join(outputDir, filename+".png")

		if verbose {
			fmt.Printf("[*] [%d/%d] Capturing PDF: %s\n", i+1, len(results), result.URL)
		}

		// Use PDF-specific capture with temp file download
		if err := screenshotter.CapturePDF(ctx, result.URL, filePath, filepath.Join(outputDir, ".temp")); err != nil {
			fmt.Printf("[!] Failed to capture PDF %s: %v\n", result.URL, err)
			continue
		}

		// Check file size
		if err := optimizeImageSize(filePath, 120); err != nil {
			fmt.Printf("[!] Failed to optimize %s: %v\n", filename, err)
		}

		if verbose {
			size, _ := getFileSize(filePath)
			fmt.Printf("[+] Screenshot saved: %s (%.2f KB)\n", filename+".png", float64(size)/1024)
		}
	}
}

// captureScreenshots captures screenshots of URLs
func captureScreenshots(ctx context.Context, results []Result, outputDir string, timeout int, verbose bool) {
	screenshotter := NewScreenshotter(timeout, verbose)
	defer screenshotter.Close()

	for i, result := range results {
		// Generate filename from URL
		filename := sanitizeFilename(result.URL)
		filePath := filepath.Join(outputDir, filename+".png")

		if verbose {
			fmt.Printf("[*] [%d/%d] Capturing: %s\n", i+1, len(results), result.URL)
		}

		if err := screenshotter.Capture(ctx, result.URL, filePath); err != nil {
			fmt.Printf("[!] Failed to capture %s: %v\n", result.URL, err)
			continue
		}

		// Check file size
		if err := optimizeImageSize(filePath, 120); err != nil {
			fmt.Printf("[!] Failed to optimize %s: %v\n", filename, err)
		}

		if verbose {
			size, _ := getFileSize(filePath)
			fmt.Printf("[+] Screenshot saved: %s (%.2f KB)\n", filename+".png", float64(size)/1024)
		}
	}
}

// sanitizeFilename creates a valid filename from URL
func sanitizeFilename(url string) string {
	// Remove protocol
	url = strings.Replace(url, "https://", "", 1)
	url = strings.Replace(url, "http://", "", 1)

	// Replace invalid characters
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	url = re.ReplaceAllString(url, "_")

	// Limit length
	if len(url) > 50 {
		url = url[:50]
	}

	return url
}

// saveResults saves search results to a file
// saveResults saves search results to a file with highlighted keywords
func saveResults(results []Result, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, result := range results {
		content := fmt.Sprintf("URL: %s\n", result.URL)
		content += fmt.Sprintf("Title: %s\n", result.Title)

		// Highlight keywords in green in the snippet
		highlightedSnippet := highlightKeywords(result.Snippet, result.Keywords)
		content += fmt.Sprintf("Snippet: %s\n", highlightedSnippet)
		content += strings.Repeat("-", 80) + "\n\n"

		if _, err := file.WriteString(content); err != nil {
			return err
		}
	}

	return nil
}

// highlightKeywords highlights keywords in green color in text
func highlightKeywords(text string, keywords []string) string {
	// ANSI color codes for green using hex escape
	const greenStart = "\x1b[32m"
	const greenEnd = "\x1b[0m"

	result := text
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		// Case-insensitive replacement with highlighting
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(keyword))
		result = re.ReplaceAllString(result, greenStart+keyword+greenEnd)
	}
	return result
}

// getFileSize returns file size in bytes
func getFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// optimizeImageSize compresses image if it exceeds max size in KB
func optimizeImageSize(filePath string, maxSizeKB int) error {
	size, err := getFileSize(filePath)
	if err != nil {
		return err
	}

	maxSizeBytes := int64(maxSizeKB * 1024)
	if size <= maxSizeBytes {
		return nil
	}

	// Use ImageMagick to compress the image
	// Convert PNG to JPEG with reduced quality
	tempFile := filePath + ".tmp.jpg"

	// Try to compress with ImageMagick
	cmd := exec.Command("convert",
		filePath,
		"-quality", "60",
		"-resize", "50%",
		tempFile)

	if err := cmd.Run(); err == nil {
		// Check if compressed version is smaller
		newSize, err := getFileSize(tempFile)
		if err == nil && newSize < maxSizeBytes {
			// Replace original with compressed version
			os.Remove(filePath)
			os.Rename(tempFile, filePath)
			return nil
		}
		os.Remove(tempFile)
	}

	// If ImageMagick didn't work, try to reduce dimensions with any available tool
	return nil
}

// truncate truncates a string to max length
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
