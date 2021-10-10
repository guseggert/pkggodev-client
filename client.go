package pkggodevclient

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

type client struct {
	httpClient *http.Client
	baseURL    string
}

var ErrNotFound = errors.New("not found on pkg.go.dev")

type ErrorList struct {
	Errs []error
}

func (e *ErrorList) Error() string {
	return fmt.Sprintf("errors: %v", e.Errs)
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

type ImportedBy struct {
	Package    string
	ImportedBy []string
}

func (c *client) ImportedBy(pkg string) (*ImportedBy, error) {
	col := c.newCollector()
	importedBy := &ImportedBy{Package: pkg}
	var err error

	col.OnHTML(".u-breakWord", func(e *colly.HTMLElement) {
		importedBy.ImportedBy = append(importedBy.ImportedBy, strings.TrimSpace(e.Text))
	})
	col.OnError(func(r *colly.Response, e error) {
		if r.StatusCode == 404 {
			err = ErrNotFound
			return
		}
		err = fmt.Errorf("making req to %s: %w", r.Request.URL.String(), e)
	})
	col.Visit(fmt.Sprintf("%s/%s?tab=importedby", c.baseURL, pkg))
	if err != nil {
		return nil, err
	}
	return importedBy, nil
}

type Package struct {
	Package                   string
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

func (c *client) DescribePackage(pkg string) (*Package, error) {
	col := c.newCollector()
	p := &Package{Package: pkg}
	errs := &ErrorList{}

	col.OnHTML("[data-test-id=UnitHeader-version]", func(e *colly.HTMLElement) {
		versionStr := e.DOM.Children().First().Text()
		version := strings.TrimSpace(strings.TrimPrefix(versionStr, "Version: "))
		p.Version = version
	})
	col.OnHTML("[data-test-id=UnitHeader-licenses]", func(e *colly.HTMLElement) {
		licenseStr := e.DOM.Children().First().Text()
		p.License = strings.TrimSpace(licenseStr)
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
		t, err := reformatDateStr(dateStr)
		if err != nil {
			errs.Errs = append(errs.Errs, err)
			return
		}
		p.Published = t
	})
	col.OnHTML(".UnitHeader-titleHeading", func(e *colly.HTMLElement) {
		for next := e.DOM.Next(); ; next = next.Next() {
			switch next.Text() {
			case "package":
				p.IsPackage = true
			case "module":
				p.IsModule = true
			default:
				// I'm not aware of a case where package=false,
				// so if we've gotten here and package=false then
				// return an error since it probably means
				// that we parsed incorrectly
				if !p.IsPackage {
					errs.Errs = append(errs.Errs, fmt.Errorf("IsPackage=false after parsing page for '%s', this probably indicates a parsing bug", pkg))
				}
				return
			}
		}
	})

	col.OnError(func(r *colly.Response, e error) {
		if r.StatusCode == 404 {
			errs.Errs = append(errs.Errs, ErrNotFound)
			return
		}
		errs.Errs = append(errs.Errs, fmt.Errorf("making req to %s: %w", r.Request.URL.String(), e))
	})
	col.Visit(fmt.Sprintf("%s/%s", c.baseURL, pkg))
	if len(errs.Errs) != 0 {
		return nil, errs
	}
	return p, nil
}

type Versions struct {
	Package  string
	Versions []Version
}
type Version struct {
	MajorVersion string
	FullVersion  string
	Date         string
}

// TODO: parse the changes and wire them up to Version
type Change struct {
	URL            string
	Symbol         string
	SymbolSynopsis string
}

// reformatDateStr takes a date string from the website like 'Jan 2, 2006'
// and returns an ISO date like '2006-01-02'.
func reformatDateStr(s string) (string, error) {
	t, err := time.Parse("Jan 2, 2006", s)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02"), nil
}

func (c *client) Versions(pkg string) (*Versions, error) {
	//https://pkg.go.dev/github.com/ipfs/ipfs-cluster/ipfsconn/ipfshttp?tab=versions
	col := c.newCollector()
	errs := &ErrorList{}

	versions := &Versions{Package: pkg}
	col.OnHTML(".Versions-list", func(e *colly.HTMLElement) {
		var curVersion Version
		var curMajorVersion string
		e.DOM.Children().Each(func(i int, s *goquery.Selection) {
			// start of a new version entry
			// this is always present, but sometimes it's empty
			if s.HasClass("Version-major") {
				mv := strings.TrimSpace(s.Text())
				// if it's empty, we just continue using the existing mv
				if mv != "" {
					curMajorVersion = mv
				}
				curVersion.MajorVersion = curMajorVersion
			}
			if s.HasClass("Version-tag") {
				version := s.Find(".js-versionLink").Text()
				curVersion.FullVersion = version
			}
			// this means there are no changes, and it's the end of the entry
			if s.HasClass("Version-commitTime") {
				dateStr := strings.TrimSpace(s.Text())
				t, err := reformatDateStr(dateStr)
				if err != nil {
					errs.Errs = append(errs.Errs, err)
					return
				}
				curVersion.Date = t
				versions.Versions = append(versions.Versions, curVersion)
				curVersion = Version{}
			}
			// this means there are changes, and it's also the end of the entry
			if s.HasClass("Version-details") {
				dateStr := strings.TrimSpace(s.Find(".Version-summary").Text())
				t, err := reformatDateStr(dateStr)
				if err != nil {
					println("error in version details: " + err.Error())
					return
				}
				curVersion.Date = t
				versions.Versions = append(versions.Versions, curVersion)
				curVersion = Version{}

				// TODO: parse the changes
			}
		})
	})

	col.OnError(func(r *colly.Response, e error) {
		if r.StatusCode == 404 {
			errs.Errs = append(errs.Errs, ErrNotFound)
			return
		}
		errs.Errs = append(errs.Errs, fmt.Errorf("making req to %s: %w", r.Request.URL.String(), e))
	})

	col.Visit(fmt.Sprintf("%s/%s?tab=versions", c.baseURL, pkg))
	return versions, nil
}

type SearchResults struct {
	Results []SearchResult
}

type SearchResult struct {
	Package    string
	Version    string
	Published  string
	ImportedBy int
	License    string
	Synopsis   string
}

func (c *client) Search(query string) (*SearchResults, error) {
	col := c.newCollector()
	results := &SearchResults{}
	errs := &ErrorList{}
	morePages := true
	col.OnHTML("[data-test-id=results-total]", func(e *colly.HTMLElement) {
		resultsStr := strings.TrimSpace(e.Text)
		if resultsStr == "0 results" {
			return
		}
		resultsSplit := strings.Split(resultsStr, " ")
		upperBound := resultsSplit[2]
		totalResults := resultsSplit[4]
		morePages = upperBound != totalResults
	})
	col.OnHTML(".LegacySearchSnippet", func(e *colly.HTMLElement) {
		pkg := strings.TrimSpace(e.DOM.Find("[data-test-id=snippet-title]").Text())
		synopsis := strings.TrimSpace(e.DOM.Find(".SearchSnippet-synopsis").Text())
		info := e.DOM.Find(".SearchSnippet-infoLabel")
		version := strings.TrimSpace(info.Find("[data-test-id=snippet-version]").Text())

		publishedDateStr := strings.TrimSpace(info.Find("[data-test-id=snippet-published]").Text())
		importedByWithCommas := strings.TrimSpace(info.Find("[data-test-id=snippet-importedby]").Text())
		importedByStr := strings.ReplaceAll(importedByWithCommas, ",", "")
		importedBy, err := strconv.Atoi(importedByStr)
		if err != nil {
			errs.Errs = append(errs.Errs, err)
			return
		}
		license := strings.TrimSpace(info.Find("[data-test-id=snippet-license]").Text())
		result := SearchResult{
			Package:    pkg,
			Synopsis:   synopsis,
			Version:    version,
			Published:  publishedDateStr,
			ImportedBy: importedBy,
			License:    license,
		}
		results.Results = append(results.Results, result)
	})
	col.OnError(func(r *colly.Response, e error) {
		errs.Errs = append(errs.Errs, e)
	})
	for page := 1; morePages; page++ {
		col.Visit(fmt.Sprintf("%s/search?q=%s&m=package&page=%d", c.baseURL, query, page))
		if len(errs.Errs) > 0 {
			return nil, errs
		}
	}

	return results, nil
}

type License struct {
	Name     string
	Source   string
	FullText string
}

func (c *client) Licenses(pkg string) ([]License, error) {
	return nil, nil
}
