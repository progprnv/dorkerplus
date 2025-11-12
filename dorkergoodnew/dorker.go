package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/option"
)

// GoogleDorker performs Google searches using Custom Search API
type GoogleDorker struct {
	config  *Config
	verbose bool
	client  *http.Client
}

// NewGoogleDorker creates a new GoogleDorker instance
func NewGoogleDorker(config *Config, verbose bool) *GoogleDorker {
	return &GoogleDorker{
		config:  config,
		verbose: verbose,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search performs a Google dork search with pagination support
func (g *GoogleDorker) Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	var results []Result

	if len(g.config.Google) == 0 {
		return nil, fmt.Errorf("no Google API credentials configured")
	}

	// Try each API credential until one works
	var lastErr error
	for i, cred := range g.config.Google {
		if g.verbose {
			fmt.Printf("[*] Trying API credential %d/%d\n", i+1, len(g.config.Google))
		}

		svc, err := customsearch.NewService(ctx, option.WithAPIKey(cred.APIKey))
		if err != nil {
			lastErr = err
			continue
		}

		// Pagination: API allows 10 results per request, max 100 results total
		// Use multiple requests to get more results
		for startIndex := 1; len(results) < maxResults; startIndex += 10 {
			resultsNeeded := maxResults - len(results)
			numToFetch := min(resultsNeeded, 10) // API allows max 10 per request

			// Perform search with pagination
			call := svc.Cse.List().
				Q(query).
				Cx(cred.SearchEngineID).
				Num(int64(numToFetch)).
				Start(int64(startIndex))

			resp, err := call.Context(ctx).Do()
			if err != nil {
				lastErr = err
				if g.verbose {
					fmt.Printf("[!] API error with credential %d (page %d): %v\n", i+1, startIndex, err)
				}
				break // Exit pagination loop, try next credential
			}

			// Check if we got any results
			if resp.Items == nil || len(resp.Items) == 0 {
				lastErr = fmt.Errorf("no more results found")
				break // No more results available
			}

			// Process results
			for _, item := range resp.Items {
				if len(results) >= maxResults {
					break
				}

				// Fetch full content from URL
				content := g.fetchContent(ctx, item.Link)

				result := Result{
					URL:     item.Link,
					Title:   item.Title,
					Snippet: item.Snippet,
					Content: content,
				}
				results = append(results, result)
			}

			// If Google returned fewer results than requested, we've reached the end
			if len(resp.Items) < numToFetch {
				break
			}
		}

		if len(results) > 0 {
			return results, nil
		}

		lastErr = fmt.Errorf("no results found")
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return results, nil
}

// fetchContent fetches the content of a URL
func (g *GoogleDorker) fetchContent(ctx context.Context, url string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := g.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Limit read size to prevent memory issues
	content, err := io.ReadAll(io.LimitReader(resp.Body, 1024*100)) // 100KB limit
	if err != nil {
		return ""
	}

	return string(content)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
