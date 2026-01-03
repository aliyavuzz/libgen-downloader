package libgen

import (
	"net/http"
	"time"
)

// Book represents a book found on Libgen
type Book struct {
	ID          string
	Title       string
	Author      string
	Publisher   string
	Year        string
	Pages       string
	Language    string
	Size        string
	Extension   string
	MD5         string
	DownloadURL string
}

// SearchResult contains search results from Libgen
type SearchResult struct {
	Books      []Book
	TotalFound int
	Query      string
	SearchTime time.Duration
}

// Client is the Libgen API client
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// SearchOptions allows customizing search behavior
type SearchOptions struct {
	Query      string
	MaxResults int
	SortBy     string // title, year, publisher, etc.
}
