package pkggodevclient

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gocolly/colly/v2"
)

type client struct {
	httpClient *http.Client
	baseURL    string
}

func New(options ...func(c *client)) *client {
	c := &client{
		baseURL: "https://pkg.go.dev",
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

func WithBaseURL(url string) func(c *client) {
	return func(c *client) {
		c.baseURL = url
	}
}

func WithHTTPClient(httpClient *http.Client) func(c *client) {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

func (c *client) newCollector() *colly.Collector {
	col := colly.NewCollector()
	if c.httpClient != nil {
		col.SetClient(c.httpClient)
	}
	return col
}

func (c *client) Imports(pkg string) ([]string, error) {
	col := c.newCollector()
	imports := []string{}
	var err error

	col.OnHTML(".u-breakWord", func(e *colly.HTMLElement) {
		imports = append(imports, strings.TrimSpace(e.Text))
	})
	col.OnError(func(r *colly.Response, e error) {
		err = fmt.Errorf("making req to %s: %w", r.Request.URL.String(), e)
	})
	col.Visit(fmt.Sprintf("%s/%s?tab=importedby", c.baseURL, pkg))
	if err != nil {
		return nil, err
	}
	return imports, nil
}
