package pkggodevclient

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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

func (c *client) ImportedBy(pkg string) ([]string, error) {
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

type Package struct {
	IsModule                  bool
	IsPackage                 bool
	Version                   string
	Published                 string
	License                   string
	HasValidGoModFile         bool
	HasRedistributableLicense bool
	HasTaggedVersion          bool
	HasStableVersion          bool
	Repository                string
}

func (c *client) Describe(pkg string) (*Package, error) {
	col := c.newCollector()
	p := &Package{}
	errs := []error{}

	col.OnHTML("[data-test-id=UnitHeader-version]", func(e *colly.HTMLElement) {
		versionStr := e.DOM.Children().First().Text()
		version := strings.TrimPrefix(versionStr, "Version: ")
		p.Version = version
	})
	col.OnHTML("[data-test-id=UnitHeader-licenses]", func(e *colly.HTMLElement) {
		licenseStr := e.DOM.Children().First().Text()
		p.License = licenseStr
	})
	// metadata are in a list, so parse it all at once
	// TODO: add some extra validation here, so we error if things change order
	col.OnHTML(".UnitMeta", func(e *colly.HTMLElement) {
		lis := e.DOM.Find("li")
		lis.Each(func(i int, s *goquery.Selection) {
			checked := s.Find("img[alt=checked]").Length() > 0
			switch i {
			case 0:
				p.HasValidGoModFile = checked
			case 1:
				p.HasRedistributableLicense = checked
			case 2:
				p.HasTaggedVersion = checked
			case 3:
				p.HasStableVersion = checked
			}
		})
	})
	col.OnHTML(".UnitMeta-repo", func(e *colly.HTMLElement) {
		text := e.DOM.Children().First().Text()
		p.Repository = strings.TrimSpace(strings.Trim(text, "\\n"))
	})
	col.OnHTML("[data-test-id=UnitHeader-commitTime]", func(e *colly.HTMLElement) {
		text := strings.TrimSpace(e.Text)
		dateStr := strings.TrimPrefix(text, "Published: ")
		t, err := time.Parse("Jan 2, 2006", dateStr)
		if err != nil {
			errs = append(errs, err)
			return
		}
		p.Published = t.Format("2006-01-02")
	})
	col.OnHTML(".UnitHeader-titleHeading", func(e *colly.HTMLElement) {
	LOOP:
		for next := e.DOM.Next(); ; {
			switch next.Text() {
			case "package":
				p.IsPackage = true
			case "module":
				p.IsModule = true
			default:
				break LOOP
			}
			next = next.Next()
		}
	})

	col.OnError(func(r *colly.Response, e error) {
		errs = append(errs, fmt.Errorf("making req to %s: %w", r.Request.URL.String(), e))
	})
	col.Visit(fmt.Sprintf("%s/%s", c.baseURL, pkg))
	if len(errs) != 0 {
		return nil, fmt.Errorf("errors: %v", errs)
	}
	return p, nil
}

type Version struct {
	Name    string
	Date    time.Time
	Changes []Change
}

type Change struct {
	// the function that changed
	Func string
	// the type containing the method, if the function is a method (else an empty string)
	Type string
}

func (c *client) Versions(pkg string) ([]Version, error) {
	return nil, nil
}

type License struct {
	Name     string
	Source   string
	FullText string
}

func (c *client) Licenses(pkg string) ([]License, error) {
	return nil, nil
}
