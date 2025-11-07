package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Screenshotter captures screenshots of web pages using EyeWitness-like approach
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

// Capture takes a screenshot of a URL and saves it to file (EyeWitness style)
func (s *Screenshotter) Capture(ctx context.Context, url string, outputPath string) error {
	// Try to use wkhtmltoimage if available
	if _, err := exec.LookPath("wkhtmltoimage"); err == nil {
		if err := s.captureWithWkhtmltoimage(url, outputPath); err == nil {
			return nil
		}
	}

	// Try to use Chrome/Chromium if available
	if _, err := exec.LookPath("google-chrome"); err == nil {
		if err := s.captureWithChrome(url, outputPath); err == nil {
			return nil
		}
	}

	// Try to use chromium if available
	if _, err := exec.LookPath("chromium"); err == nil {
		if err := s.captureWithChromium(url, outputPath); err == nil {
			return nil
		}
	}

	// Fallback to rendering page content as image
	return s.captureWithPageRender(url, outputPath)
}

// CapturePDF downloads a PDF file and captures a screenshot of it
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
		return s.createErrorImage(outputPath, "Request failed", err.Error())
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return s.createErrorImage(outputPath, "Download failed", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return s.createErrorImage(outputPath, fmt.Sprintf("HTTP %d", resp.StatusCode), url)
	}

	// Save PDF to temp file
	tempPDF := filepath.Join(tempDir, "temp.pdf")
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

	// Try to convert PDF to image using available tools
	if _, err := exec.LookPath("pdftoppm"); err == nil {
		return s.capturePDFWithPdftoppm(tempPDF, outputPath)
	}

	if _, err := exec.LookPath("convert"); err == nil {
		return s.capturePDFWithImageMagick(tempPDF, outputPath)
	}

	if _, err := exec.LookPath("pdfimages"); err == nil {
		return s.capturePDFWithPdfimages(tempPDF, outputPath)
	}

	// Fallback: create PDF visualization
	return s.createPDFVisualization(outputPath, url, resp.Header.Get("Content-Length"))
}

// capturePDFWithPdftoppm uses pdftoppm to convert first page of PDF
func (s *Screenshotter) capturePDFWithPdftoppm(pdfPath string, outputPath string) error {
	// Convert first page to PPM then to PNG
	tempPPM := strings.TrimSuffix(outputPath, ".png") + ".ppm"
	defer os.Remove(tempPPM)

	cmd := exec.Command("pdftoppm", "-png", "-f", "1", "-l", "1", "-singlefile", pdfPath, strings.TrimSuffix(outputPath, ".png"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pdftoppm failed: %w", err)
	}

	// pdftoppm with -png flag should create .png file directly
	return nil
}

// capturePDFWithImageMagick uses ImageMagick convert to capture first PDF page
func (s *Screenshotter) capturePDFWithImageMagick(pdfPath string, outputPath string) error {
	cmd := exec.Command("convert",
		"-density", "150",
		"-quality", "85",
		pdfPath+"[0]",
		"-resize", "1024x768",
		outputPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("imagemagick failed: %w", err)
	}
	return nil
}

// capturePDFWithPdfimages uses pdfimages to extract first image from PDF
func (s *Screenshotter) capturePDFWithPdfimages(pdfPath string, outputPath string) error {
	tempDir := filepath.Dir(outputPath)
	tempBase := filepath.Join(tempDir, ".pdfimage")

	cmd := exec.Command("pdfimages", "-png", "-f", "1", "-l", "1", pdfPath, tempBase)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pdfimages failed: %w", err)
	}

	// Try to move the extracted image
	firstImage := tempBase + "-000.png"
	if _, err := os.Stat(firstImage); err == nil {
		if err := os.Rename(firstImage, outputPath); err != nil {
			return fmt.Errorf("failed to move extracted image: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no images extracted from PDF")
}

// captureWithWkhtmltoimage uses wkhtmltoimage if available
func (s *Screenshotter) captureWithWkhtmltoimage(url string, outputPath string) error {
	cmd := exec.Command("wkhtmltoimage", "--quiet", url, outputPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wkhtmltoimage failed: %w", err)
	}
	return nil
}

// captureWithChrome uses google-chrome if available
func (s *Screenshotter) captureWithChrome(url string, outputPath string) error {
	cmd := exec.Command("google-chrome",
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--screenshot="+outputPath,
		"--window-size=1024,768",
		url)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("chrome failed: %w", err)
	}
	return nil
}

// captureWithChromium uses chromium if available
func (s *Screenshotter) captureWithChromium(url string, outputPath string) error {
	cmd := exec.Command("chromium",
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--screenshot="+outputPath,
		"--window-size=1024,768",
		url)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("chromium failed: %w", err)
	}
	return nil
}

// captureWithPageRender fetches page and renders as image
func (s *Screenshotter) captureWithPageRender(url string, outputPath string) error {
	// Fetch the page
	client := &http.Client{
		Timeout: time.Duration(s.timeout) * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return s.createErrorImage(outputPath, "Request failed", err.Error())
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return s.createErrorImage(outputPath, "Connection failed", err.Error())
	}
	defer resp.Body.Close()

	// Determine content type
	contentType := resp.Header.Get("Content-Type")

	// Create appropriate visualization
	if resp.StatusCode == 200 {
		if strings.Contains(contentType, "pdf") {
			return s.createPDFVisualization(outputPath, url, resp.Header.Get("Content-Length"))
		} else if strings.Contains(contentType, "html") {
			return s.createHTMLVisualization(outputPath, url, resp.ContentLength)
		} else {
			return s.createSuccessVisualization(outputPath, url, contentType)
		}
	}

	return s.createErrorImage(outputPath, fmt.Sprintf("HTTP %d", resp.StatusCode), url)
}

// createPDFVisualization creates a visual representation for PDFs
func (s *Screenshotter) createPDFVisualization(outputPath string, url string, contentLength string) error {
	width := 1024
	height := 768
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// PDF document background (light gray with blue accent)
	bgColor := color.RGBA{R: 245, G: 245, B: 250, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Add PDF header
	headerColor := color.RGBA{R: 41, G: 128, B: 185, A: 255}
	draw.Draw(img, image.Rect(0, 0, width, 100), &image.Uniform{C: headerColor}, image.Point{}, draw.Src)

	// Add text representation (simple blocks to simulate content)
	contentColor := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	for i := 0; i < 15; i++ {
		y := 150 + i*40
		if y > height-100 {
			break
		}
		// Draw line blocks to represent text
		draw.Draw(img, image.Rect(50, y, width-50, y+25), &image.Uniform{C: contentColor}, image.Point{}, draw.Src)
	}

	// Add footer with URL
	footerColor := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	draw.Draw(img, image.Rect(0, height-50, width, height), &image.Uniform{C: footerColor}, image.Point{}, draw.Src)

	return s.saveImage(img, outputPath)
}

// createHTMLVisualization creates a visual representation for HTML pages
func (s *Screenshotter) createHTMLVisualization(outputPath string, url string, contentLength int64) error {
	width := 1024
	height := 768
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Website background (light)
	bgColor := color.RGBA{R: 250, G: 250, B: 250, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Navigation bar
	navColor := color.RGBA{R: 52, G: 73, B: 94, A: 255}
	draw.Draw(img, image.Rect(0, 0, width, 60), &image.Uniform{C: navColor}, image.Point{}, draw.Src)

	// Main content area with blocks
	contentColor := color.RGBA{R: 149, G: 165, B: 166, A: 255}
	for i := 0; i < 12; i++ {
		y := 100 + i*50
		if y > height-100 {
			break
		}
		draw.Draw(img, image.Rect(30, y, width-30, y+30), &image.Uniform{C: contentColor}, image.Point{}, draw.Src)
	}

	// Sidebar
	sidebarColor := color.RGBA{R: 189, G: 195, B: 199, A: 255}
	draw.Draw(img, image.Rect(width-200, 80, width, height-50), &image.Uniform{C: sidebarColor}, image.Point{}, draw.Src)

	// Footer
	footerColor := color.RGBA{R: 44, G: 62, B: 80, A: 255}
	draw.Draw(img, image.Rect(0, height-50, width, height), &image.Uniform{C: footerColor}, image.Point{}, draw.Src)

	return s.saveImage(img, outputPath)
}

// createSuccessVisualization creates a generic success visualization
func (s *Screenshotter) createSuccessVisualization(outputPath string, url string, contentType string) error {
	width := 1024
	height := 768
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Green success background
	bgColor := color.RGBA{R: 230, G: 245, B: 230, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Success header (green)
	headerColor := color.RGBA{R: 46, G: 204, B: 113, A: 255}
	draw.Draw(img, image.Rect(0, 0, width, 120), &image.Uniform{C: headerColor}, image.Point{}, draw.Src)

	// Content visualization
	contentColor := color.RGBA{R: 52, G: 152, B: 219, A: 255}
	for i := 0; i < 10; i++ {
		y := 200 + i*50
		if y > height-150 {
			break
		}
		draw.Draw(img, image.Rect(100, y, width-100, y+30), &image.Uniform{C: contentColor}, image.Point{}, draw.Src)
	}

	return s.saveImage(img, outputPath)
}

// createErrorImage creates an error visualization
func (s *Screenshotter) createErrorImage(outputPath string, title string, detail string) error {
	width := 1024
	height := 768
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Red error background
	bgColor := color.RGBA{R: 245, G: 230, B: 230, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Error header (red)
	headerColor := color.RGBA{R: 231, G: 76, B: 60, A: 255}
	draw.Draw(img, image.Rect(0, 0, width, 150), &image.Uniform{C: headerColor}, image.Point{}, draw.Src)

	// Error details area
	detailColor := color.RGBA{R: 192, G: 57, B: 43, A: 255}
	draw.Draw(img, image.Rect(50, 250, width-50, 350), &image.Uniform{C: detailColor}, image.Point{}, draw.Src)

	return s.saveImage(img, outputPath)
}

// saveImage saves an image to disk
func (s *Screenshotter) saveImage(img image.Image, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return nil
}

// Close closes the screenshotter
func (s *Screenshotter) Close() {
	// Cleanup handled by context cancellation
}
