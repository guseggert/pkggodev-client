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

type ImportedByRequest struct {
	Package string
}

type ImportedBy struct {
	Package    string
	ImportedBy []string
}

func (c *client) ImportedBy(req ImportedByRequest) (*ImportedBy, error) {
	col := c.newCollector()
	importedBy := &ImportedBy{Package: req.Package}
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
	col.Visit(fmt.Sprintf("%s/%s?tab=importedby", c.baseURL, req.Package))
	if err != nil {
		return nil, err
	}
	return importedBy, nil
}

type DescribePackageRequest struct {
	Package string
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

func (c *client) DescribePackage(req DescribePackageRequest) (*Package, error) {
	col := c.newCollector()
	p := &Package{Package: req.Package}
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
		t, err := normalizeTime(dateStr)
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
					errs.Errs = append(errs.Errs, fmt.Errorf("IsPackage=false after parsing page for '%s', this probably indicates a parsing bug", req.Package))
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
	col.Visit(fmt.Sprintf("%s/%s", c.baseURL, req.Package))
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

// normalizeTime normalizes a time string into a consistent format.
// Times are generally represented as dates, so this only returns a date string, not a time string.
// It handles cases of durations as well like '1 hour ago' which are sometimes used (like in search results).
// Parsed durations are returned as times relative to now.
func normalizeTime(s string) (string, error) {
	var absTime time.Time

	// as far as I can tell, the UI only uses "<quantity> hours ago" or "<quantity> days ago"
	// e.g. "2 hours ago", "1 hours ago", "0 hours ago", "5 days ago", etc.
	// at some point it switches back to an absolute date
	// you can find examples at https://index.golang.org/index?since=2021-10-10T09:08:52.997264Z
	if s == "today" {
		absTime = time.Now()
	} else if strings.Contains(s, "ago") {
		now := time.Now()
		// <quantity> <unit>[s] ago
		split := strings.Split(s, " ")
		quantityStr := split[0]
		quantity, err := strconv.ParseInt(quantityStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("parsing quantity '%s' of time '%s': %w", quantityStr, s, err)
		}
		quantityDur := time.Duration(quantity)
		unit := strings.TrimSuffix(split[1], "s")

		switch unit {
		case "hour":
			absTime = now.Add(-quantityDur * time.Hour)
		case "day":
			absTime = now.AddDate(0, 0, -int(quantity))
		case "week":
			absTime = now.AddDate(0, 0, -7*int(quantity))
		default:
			return "", fmt.Errorf("unknown quantity '%s' when parsing '%s'", quantityStr, s)
		}
	} else {
		d, err := time.Parse("Jan 2, 2006", s)
		if err != nil {
			return "", fmt.Errorf("parsing date '%s': %w", s, err)
		}
		absTime = d
	}
	return absTime.Format("2006-01-02"), nil
}

type VersionsRequest struct {
	Package string
}

func (c *client) Versions(req VersionsRequest) (*Versions, error) {
	//https://pkg.go.dev/github.com/ipfs/ipfs-cluster/ipfsconn/ipfshttp?tab=versions
	col := c.newCollector()
	errs := &ErrorList{}

	versions := &Versions{Package: req.Package}
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
				t, err := normalizeTime(dateStr)
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
				t, err := normalizeTime(dateStr)
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

	col.Visit(fmt.Sprintf("%s/%s?tab=versions", c.baseURL, req.Package))
	return versions, nil
}

type SearchRequest struct {
	Query string
	Limit int
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

func (c *client) Search(req SearchRequest) (*SearchResults, error) {
	col := c.newCollector()
	results := &SearchResults{}
	errs := &ErrorList{}

	morePages := true

	// on page n, compute if we should follow to page n+1
	col.OnHTML("[data-test-id=results-total]", func(e *colly.HTMLElement) {
		resultsStr := strings.TrimSpace(e.Text)
		if resultsStr == "0 results" {
			return
		}
		resultsSplit := strings.Split(resultsStr, " ")
		if len(resultsSplit) == 2 {
			// e.g. "1 result", "2 results", ...
			morePages = false
		} else {
			// e.g. "1 - 25 of 125 results", "26-50 of 125 results", ...
			upperBoundStr := resultsSplit[2]
			upperBound, err := strconv.Atoi(upperBoundStr)
			if err != nil {
				errs.Errs = append(errs.Errs, err)
				return
			}

			totalResultsStr := resultsSplit[4]

			reachedReqLimit := upperBound >= req.Limit
			reachedLastPage := upperBoundStr == totalResultsStr
			morePages = !reachedReqLimit && !reachedLastPage
		}
	})

	col.OnHTML(".LegacySearchSnippet", func(e *colly.HTMLElement) {
		if len(results.Results) == req.Limit {
			return
		}

		pkg := strings.TrimSpace(e.DOM.Find("[data-test-id=snippet-title]").Text())
		synopsis := strings.TrimSpace(e.DOM.Find(".SearchSnippet-synopsis").Text())
		info := e.DOM.Find(".SearchSnippet-infoLabel")

		// pseudoversions are truncated and contain '...', so we have to do an additional lookup
		// it is of the format "v0.0.0-...-<hash>" so we can be confident in the lookup
		// for now we just leave version blank if it's a pseudoversion, so searches don't take a long time
		// we can add a flag to turn on version info later
		// TODO: add flag to turn on inclusion of full pseudoversions
		version := strings.TrimSpace(info.Find("[data-test-id=snippet-version]").Text())

		publishedDateStr := strings.TrimSpace(info.Find("[data-test-id=snippet-published]").Text())
		published, err := normalizeTime(publishedDateStr)
		if err != nil {
			errs.Errs = append(errs.Errs, err)
			return
		}
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
			Published:  published,
			ImportedBy: importedBy,
			License:    license,
		}
		results.Results = append(results.Results, result)
	})
	col.OnError(func(r *colly.Response, e error) {
		errs.Errs = append(errs.Errs, e)
	})
	for page := 1; morePages; page++ {
		col.Visit(fmt.Sprintf("%s/search?q=%s&m=package&page=%d", c.baseURL, req.Query, page))
		if len(errs.Errs) > 0 {
			return nil, errs
		}
	}

	return results, nil
}

type ImportsRequest struct {
	Package string
}

type Imports struct {
	Package                string
	Imports                []string
	ModuleImports          map[string][]string
	StandardLibraryImports []string
}

func (c *client) Imports(req ImportsRequest) (*Imports, error) {
	return nil, nil
}

type LicensesRequest struct {
	Package string
}

type License struct {
	Name     string
	Source   string
	FullText string
}

func (c *client) Licenses(req LicensesRequest) ([]License, error) {
	return nil, nil
}
