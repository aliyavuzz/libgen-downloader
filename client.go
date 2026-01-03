package libgen

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	DefaultBaseURL   = "https://libgen.is"
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
)

// NewClient creates a new Libgen client
func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent: DefaultUserAgent,
	}
}

// Search searches for books on Libgen
func (c *Client) Search(opts SearchOptions) (*SearchResult, error) {
	start := time.Now()

	if opts.Query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	if opts.MaxResults == 0 {
		opts.MaxResults = 25
	}

	// Build search URL
	searchURL := fmt.Sprintf("%s/search.php?req=%s&res=%d",
		c.BaseURL,
		url.QueryEscape(opts.Query),
		opts.MaxResults,
	)

	// Make HTTP request
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	// Parse HTML response
	books, err := c.parseSearchResults(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse results: %w", err)
	}

	return &SearchResult{
		Books:      books,
		TotalFound: len(books),
		Query:      opts.Query,
		SearchTime: time.Since(start),
	}, nil
}

// parseSearchResults parses the HTML search results page
func (c *Client) parseSearchResults(body io.Reader) ([]Book, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, err
	}

	var books []Book

	// Find the results table
	doc.Find("table.c tbody tr").Each(func(i int, row *goquery.Selection) {
		// Skip header rows
		if i == 0 {
			return
		}

		book := Book{}

		// Parse columns
		row.Find("td").Each(func(colIndex int, col *goquery.Selection) {
			text := strings.TrimSpace(col.Text())

			switch colIndex {
			case 0: // ID
				book.ID = text
			case 1: // Author
				book.Author = text
			case 2: // Title
				book.Title = text
				// Extract download link if available
				col.Find("a[href*='md5']").Each(func(_ int, link *goquery.Selection) {
					if href, exists := link.Attr("href"); exists {
						book.MD5 = extractMD5FromURL(href)
					}
				})
			case 3: // Publisher
				book.Publisher = text
			case 4: // Year
				book.Year = text
			case 5: // Pages
				book.Pages = text
			case 6: // Language
				book.Language = text
			case 7: // Size
				book.Size = text
			case 8: // Extension
				book.Extension = text
			case 9: // Mirrors - download links
				col.Find("a").Each(func(_ int, link *goquery.Selection) {
					if href, exists := link.Attr("href"); exists {
						if strings.Contains(href, "library.lol") ||
							strings.Contains(href, "libgen.li") {
							book.DownloadURL = href
						}
					}
				})
			}
		})

		// Only add if we have essential information
		if book.Title != "" && book.MD5 != "" {
			// If no direct download URL, construct it from MD5
			if book.DownloadURL == "" {
				book.DownloadURL = c.getDownloadURL(book.MD5)
			}
			books = append(books, book)
		}
	})

	return books, nil
}

// getDownloadURL constructs a download URL from MD5 hash
func (c *Client) getDownloadURL(md5 string) string {
	// Try library.lol first (usually most reliable)
	return fmt.Sprintf("https://library.lol/main/%s", md5)
}

// extractMD5FromURL extracts MD5 hash from a Libgen URL
func extractMD5FromURL(urlStr string) string {
	if strings.Contains(urlStr, "md5=") {
		parts := strings.Split(urlStr, "md5=")
		if len(parts) > 1 {
			return strings.Split(parts[1], "&")[0]
		}
	}
	return ""
}

// GetDirectDownloadLink follows redirects to get the actual download link
func (c *Client) GetDirectDownloadLink(book Book) (string, error) {
	if book.DownloadURL == "" {
		return "", fmt.Errorf("no download URL available")
	}

	req, err := http.NewRequest("GET", book.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Parse the page to find actual download link
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Look for download button/link
	var directLink string
	doc.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		text := strings.ToLower(link.Text())

		// Common patterns for download links
		if strings.Contains(text, "get") ||
			strings.Contains(text, "download") ||
			strings.Contains(href, "/get.php") {
			if strings.HasPrefix(href, "http") {
				directLink = href
			} else if strings.HasPrefix(href, "/") {
				directLink = "https://library.lol" + href
			}
		}
	})

	if directLink == "" {
		return "", fmt.Errorf("could not find direct download link")
	}

	return directLink, nil
}
