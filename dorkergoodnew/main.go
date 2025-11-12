package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Result represents a search result
type Result struct {
	URL          string
	Title        string
	Snippet      string
	Content      string
	MatchingLine string
	Keywords     []string
}

// Screenshotter captures screenshots of web pages
type Screenshotter struct {
	timeout int
	verbose bool
}

// NewScreenshotter creates a new Screenshotter instance
func NewScreenshotter(timeout int, verbose bool) *Screenshotter {
	return &Screenshotter{
		timeout: timeout,
		verbose: verbose,
	}
}

// Capture downloads and saves webpage content
func (s *Screenshotter) Capture(ctx context.Context, url string, outputPath string) error {
	return s.capturePageContent(ctx, url, outputPath)
}

// capturePageContent fetches page content and saves it
func (s *Screenshotter) capturePageContent(ctx context.Context, url string, outputPath string) error {
	client := &http.Client{
		Timeout: time.Duration(s.timeout) * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return s.createErrorFile(outputPath, "Request failed", err.Error())
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return s.createErrorFile(outputPath, "Connection failed", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return s.createErrorFile(outputPath, fmt.Sprintf("HTTP %d", resp.StatusCode), url)
	}

	// Read content
	content, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return s.createErrorFile(outputPath, "Read failed", err.Error())
	}

	// Save content as file
	return s.saveContent(outputPath, content, resp.Header.Get("Content-Type"), url)
}

// CapturePDF downloads a PDF file and converts it to image
func (s *Screenshotter) CapturePDF(ctx context.Context, url string, outputPath string, tempDir string) error {
	// Create temp directory if needed
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Download PDF
	client := &http.Client{
		Timeout: time.Duration(s.timeout) * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return s.createErrorFile(outputPath, "Request failed", err.Error())
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return s.createErrorFile(outputPath, "Download failed", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return s.createErrorFile(outputPath, fmt.Sprintf("HTTP %d", resp.StatusCode), url)
	}

	// Save PDF to temp file
	tempPDF := filepath.Join(tempDir, "temp_"+filepath.Base(url)+".pdf")
	tmpFile, err := os.Create(tempPDF)
	if err != nil {
		return fmt.Errorf("failed to create temp PDF: %w", err)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to save temp PDF: %w", err)
	}
	defer os.Remove(tempPDF)

	// Convert PDF to PNG image
	imagePath := outputPath + ".png"

	// Try pdftoppm first (best quality)
	if _, err := exec.LookPath("pdftoppm"); err == nil {
		cmd := exec.Command("pdftoppm", "-png", "-f", "1", "-l", "1", "-singlefile", tempPDF, outputPath)
		if err := cmd.Run(); err == nil {
			// Compress if needed
			if err := compressImage(imagePath, 100); err == nil {
				if s.verbose {
					size, _ := getFileSize(imagePath)
					fmt.Printf("[+] PDF converted: %s (%.1f KB)\n", filepath.Base(imagePath), float64(size)/1024)
				}
				return nil
			}
		}
	}

	// Try ImageMagick convert
	if _, err := exec.LookPath("convert"); err == nil {
		cmd := exec.Command("convert",
			"-density", "150",
			"-quality", "70",
			tempPDF+"[0]",
			"-resize", "1024x768",
			"-strip",
			imagePath)
		if err := cmd.Run(); err == nil {
			if err := compressImage(imagePath, 100); err == nil {
				if s.verbose {
					size, _ := getFileSize(imagePath)
					fmt.Printf("[+] PDF converted: %s (%.1f KB)\n", filepath.Base(imagePath), float64(size)/1024)
				}
				return nil
			}
		}
	}

	// Try Ghostscript
	if _, err := exec.LookPath("gs"); err == nil {
		cmd := exec.Command("gs",
			"-q",
			"-dNOPAUSE",
			"-dBATCH",
			"-dSAFER",
			"-sDEVICE=jpeg",
			"-dFirstPage=1",
			"-dLastPage=1",
			"-dJPEGQ=70",
			"-r150",
			"-sOutputFile="+imagePath,
			tempPDF)
		if err := cmd.Run(); err == nil {
			if err := compressImage(imagePath, 100); err == nil {
				if s.verbose {
					size, _ := getFileSize(imagePath)
					fmt.Printf("[+] PDF converted: %s (%.1f KB)\n", filepath.Base(imagePath), float64(size)/1024)
				}
				return nil
			}
		}
	}

	if s.verbose {
		fmt.Printf("[!] Could not convert PDF: %s\n", filepath.Base(tempPDF))
	}
	return fmt.Errorf("no PDF conversion tools available")
}

// saveContent saves fetched content to file
func (s *Screenshotter) saveContent(outputPath string, content []byte, contentType string, url string) error {
	// Determine file extension based on content type
	ext := ".html"
	if contentType != "" {
		if contains(contentType, "pdf") {
			ext = ".pdf"
		} else if contains(contentType, "json") {
			ext = ".json"
		} else if contains(contentType, "text") {
			ext = ".txt"
		}
	}

	// Create output path with extension
	fullPath := outputPath + ext

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write content to file
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if s.verbose {
		fmt.Printf("[+] Saved: %s (%d bytes)\n", fullPath, len(content))
	}

	return nil
}

// createErrorFile saves error info to a text file
func (s *Screenshotter) createErrorFile(outputPath string, title string, detail string) error {
	content := fmt.Sprintf("ERROR: %s\nDetails: %s\n", title, detail)
	return os.WriteFile(outputPath+".err", []byte(content), 0644)
}

// contains checks if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Close closes the screenshotter
func (s *Screenshotter) Close() {
	// Cleanup handled by context cancellation
}

// extractTargetDomain extracts the target domain from a dork query
func extractTargetDomain(query string) string {
	// Look for "site:" operator
	parts := strings.Fields(query)
	for _, part := range parts {
		if strings.HasPrefix(strings.ToLower(part), "site:") {
			domain := strings.TrimPrefix(strings.ToLower(part), "site:")
			// Remove any special characters and normalize
			domain = strings.Trim(domain, "\"'")
			// Keep only alphanumeric, dots, and hyphens
			re := regexp.MustCompile(`[^a-z0-9.-]`)
			domain = re.ReplaceAllString(domain, "")
			if domain != "" {
				return domain
			}
		}
	}
	return ""
}

func main() {
	// Define flags
	query := flag.String("q", "", "Google dork query (required)")
	output := flag.String("o", "", "Output file to save results (optional, auto-generated if not provided)")
	screenshotDir := flag.String("screenshots", "", "Directory to save screenshots (optional, auto-generated if not provided)")
	maxResults := flag.Int("max", 10, "Maximum number of results")
	quiet := flag.Bool("quiet", false, "Disable verbose output")
	timeout := flag.Int("timeout", 10, "Timeout in seconds for screenshot capture")
	pdfMode := flag.Bool("pdf", false, "PDF mode: download PDFs only")

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

	// Extract target domain from query
	targetDomain := extractTargetDomain(*query)
	if targetDomain == "" {
		targetDomain = "unknown_target"
	}

	// Create results directory structure
	resultsBaseDir := "results"
	targetDir := filepath.Join(resultsBaseDir, targetDomain)

	// Create directories if they don't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to create results directory: %v\n", err)
		os.Exit(1)
	}

	// Set output file if not provided
	if *output == "" {
		*output = filepath.Join(targetDir, "results.txt")
	}

	// Set screenshot directory if not provided
	var screenshotPath string
	if *screenshotDir == "" {
		screenshotPath = filepath.Join(targetDir, "screenshots")
	} else {
		screenshotPath = *screenshotDir
	}

	if verbose {
		fmt.Printf("[+] Results directory: %s\n", targetDir)
		fmt.Printf("[+] Output file: %s\n", *output)
		if *pdfMode {
			fmt.Printf("[+] Screenshot/PDF directory: %s\n", screenshotPath)
		}
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
		// For PDFs and other content, try snippet first, then content
		if results[i].Snippet != "" {
			results[i].MatchingLine = extractMatchingLine(results[i].Snippet, keywords)
		}
		if results[i].MatchingLine == "" && results[i].Content != "" {
			results[i].MatchingLine = extractMatchingLine(results[i].Content, keywords)
		}
		results[i].Keywords = keywords
		if verbose && results[i].MatchingLine != "" {
			// Highlight keywords in green in terminal output
			highlightedLine := highlightKeywords(results[i].MatchingLine, keywords)
			fmt.Printf("      Matching line: %s...\n", truncate(highlightedLine, 80))
		}
	}

	// Create screenshot directory if needed
	if err := os.MkdirAll(screenshotPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to create screenshot directory: %v\n", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Printf("[+] Screenshot directory created: %s\n", screenshotPath)
	}

	// Capture screenshots if requested
	if *pdfMode {
		fmt.Println("[*] PDF Mode: Capturing PDFs only...")
		captureScreenshotsWithPDF(ctx, results, screenshotPath, *timeout, verbose)
	} else {
		fmt.Println("[*] Capturing content...")
		captureScreenshots(ctx, results, screenshotPath, *timeout, verbose)
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

// captureScreenshotsWithPDF downloads PDFs only
func captureScreenshotsWithPDF(ctx context.Context, results []Result, outputDir string, timeout int, verbose bool) {
	screenshotter := NewScreenshotter(timeout, verbose)
	defer screenshotter.Close()

	for i, result := range results {
		// Generate filename from URL
		filename := sanitizeFilename(result.URL)
		filePath := filepath.Join(outputDir, filename)

		if verbose {
			fmt.Printf("[*] [%d/%d] Capturing PDF: %s\n", i+1, len(results), result.URL)
		}

		// Use PDF-specific capture
		if err := screenshotter.CapturePDF(ctx, result.URL, filePath, filepath.Join(outputDir, ".temp")); err != nil {
			fmt.Printf("[!] Failed to capture PDF %s: %v\n", result.URL, err)
			continue
		}

		if verbose {
			size, _ := getFileSize(filePath + ".pdf")
			// Truncate BEFORE highlighting to avoid cutting color codes
			truncatedLine := truncate(result.MatchingLine, 70)
			highlightedLine := highlightKeywords(truncatedLine, result.Keywords)
			fmt.Printf("[+] PDF converted: %s (%.1f KB) - %s\n", filename, float64(size)/1024, highlightedLine)
		}
	}
}

// captureScreenshots captures content of URLs
func captureScreenshots(ctx context.Context, results []Result, outputDir string, timeout int, verbose bool) {
	screenshotter := NewScreenshotter(timeout, verbose)
	defer screenshotter.Close()

	for i, result := range results {
		// Generate filename from URL
		filename := sanitizeFilename(result.URL)
		filePath := filepath.Join(outputDir, filename)

		if verbose {
			fmt.Printf("[*] [%d/%d] Capturing: %s\n", i+1, len(results), result.URL)
		}

		if err := screenshotter.Capture(ctx, result.URL, filePath); err != nil {
			fmt.Printf("[!] Failed to capture %s: %v\n", result.URL, err)
			continue
		}

		if verbose {
			size, _ := getFileSize(filePath)
			// Highlight keywords in the matching line for terminal output
			highlightedLine := highlightKeywords(result.MatchingLine, result.Keywords)
			fmt.Printf("[+] Content saved: %s (%.1f KB) - %s\n", filename, float64(size)/1024, truncate(highlightedLine, 60))
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
		content += fmt.Sprintf("Matching Line: %s\n", result.MatchingLine)
		content += strings.Repeat("-", 80) + "\n\n"

		if _, err := file.WriteString(content); err != nil {
			return err
		}
	}

	return nil
}

// highlightKeywords highlights keywords in green color in text
func highlightKeywords(text string, keywords []string) string {
	// ANSI color codes for green
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

// truncate truncates a string to max length
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// compressImage compresses an image file to be under maxSizeKB
func compressImage(imagePath string, maxSizeKB int) error {
	size, err := getFileSize(imagePath)
	if err != nil {
		return err
	}

	maxSizeBytes := int64(maxSizeKB * 1024)
	if size <= maxSizeBytes {
		return nil // Already small enough
	}

	// Image is too large, try to compress with ImageMagick
	if _, err := exec.LookPath("convert"); err == nil {
		tempFile := imagePath + ".tmp.jpg"

		// Reduce quality and size
		cmd := exec.Command("convert",
			imagePath,
			"-quality", "50",
			"-resize", "80%",
			"-strip",
			tempFile)

		if err := cmd.Run(); err == nil {
			newSize, err := getFileSize(tempFile)
			if err == nil && newSize < maxSizeBytes {
				os.Remove(imagePath)
				os.Rename(tempFile, imagePath)
				return nil
			}
			os.Remove(tempFile)
		}
	}

	// If still too large, try more aggressive compression
	if _, err := exec.LookPath("convert"); err == nil {
		tempFile := imagePath + ".tmp2.jpg"

		cmd := exec.Command("convert",
			imagePath,
			"-quality", "30",
			"-resize", "50%",
			"-strip",
			"-define", "jpeg:dct-method=float",
			tempFile)

		if err := cmd.Run(); err == nil {
			newSize, err := getFileSize(tempFile)
			if err == nil && newSize < maxSizeBytes {
				os.Remove(imagePath)
				os.Rename(tempFile, imagePath)
				return nil
			}
			os.Remove(tempFile)
		}
	}

	return nil
}
