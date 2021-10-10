package pkggodevclient

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func withHTTPServer(pattern string, handler http.HandlerFunc, f func(addr string)) {
	m := http.NewServeMux()
	m.HandleFunc(pattern, handler)
	// use a random ephemeral port, which is passed into the func
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	shutdown := &sync.WaitGroup{}
	shutdown.Add(1)
	stopped := &sync.WaitGroup{}
	stopped.Add(2)
	srv := &http.Server{Handler: handler}
	go func() {
		err := srv.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			println("err")
		}
		stopped.Done()
	}()
	go func() {
		shutdown.Wait()
		err := srv.Close()
		if err != nil {
			panic(err)
		}
		stopped.Done()
	}()
	f(listener.Addr().String())
	shutdown.Done()
	stopped.Wait()
}

func TestClient_ImportedBy(t *testing.T) {
	cases := []struct {
		name              string
		html              string
		httpErrCode       int
		expectImports     []string
		expectErrContains string
	}{
		{

			name: "happy case",
			html: `
<html><body>
<div class="u-breakWord">foo</div>
<div class="u-breakWord">bar</div>
</body></html>
`,
			expectImports: []string{"foo", "bar"},
		},
		{
			name:          "no results",
			html:          "",
			expectImports: nil,
		},
		{
			name:              "http error returns an error",
			html:              "",
			httpErrCode:       500,
			expectErrContains: "Internal Server Error",
		},
		{
			name:              "returns error on 404",
			httpErrCode:       404,
			expectErrContains: "not found on pkg.go.dev",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withHTTPServer("/", func(rw http.ResponseWriter, r *http.Request) {
				if c.httpErrCode != 0 {
					rw.WriteHeader(c.httpErrCode)
					return
				}
				rw.Write([]byte(c.html))
			}, func(addr string) {
				client := New(WithBaseURL("http://" + addr))
				importedBy, err := client.ImportedBy("somepackage")
				if c.expectErrContains != "" {
					assert.Contains(t, err.Error(), c.expectErrContains)
					return
				}
				assert.NoError(t, err)
				assert.Equal(t, "somepackage", importedBy.Package)
				assert.Equal(t, c.expectImports, importedBy.ImportedBy)
			})
		})
	}
}

func TestClient_DescribePackage(t *testing.T) {
	cases := []struct {
		name              string
		html              string
		httpCode          int
		expectErrContains string
		expectPackage     Package
	}{
		{
			name: "happy case",
			html: `
<html>
<div data-test-id="UnitHeader-version"><div>  fooversion  </div>
<div data-test-id="UnitHeader-licenses"><div>  foolicense  </div>
<div class="UnitMeta"><ul>
  <li><img alt="checked"/></li>
  <li><img alt="checked"/></li>
  <li><img alt="checked"/></li>
  <li><img alt="checked"/></li>
</ul></div>
<div class="UnitMeta-repo"><div>
    foorepo    </div>
<div data-test-id="UnitHeader-commitTime">  Published: Feb 3, 2000 </div>
<div class="UnitHeader-titleHeading">Heading</div>
<div>package</div>
<div>module</div>
</div></html>`,
			expectPackage: Package{
				Package:                   "somepackage",
				Version:                   "fooversion",
				License:                   "foolicense",
				HasValidGoModFile:         true,
				HasRedistributableLicense: true,
				HasTaggedVersion:          true,
				HasStableVersion:          true,
				Repository:                "foorepo",
				Published:                 "2000-02-03",
				IsModule:                  true,
				IsPackage:                 true,
			},
		},
		{
			name:          "package but not module",
			html:          `<div class="UnitHeader-titleHeading">Heading</div><div>package</div><div>something else</div>`,
			expectPackage: Package{Package: "somepackage", IsPackage: true},
		},
		{
			name:              "returns an error if IsPackage is false",
			html:              `<div class="UnitHeader-titleHeading">Heading</div><div>module</div><div>something else</div>`,
			expectErrContains: "IsPackage=false after parsing page for 'somepackage', this probably indicates a parsing bug",
		},
		{
			name:              "returns an error if HTTP req fails",
			httpCode:          500,
			expectErrContains: "Internal Server Error",
		},
		{
			name:              "returns an error if the published date can't be parsed",
			html:              `<div data-test-id="UnitHeader-commitTime">  Published: February 333, 20 </div>`,
			expectErrContains: "parsing time",
		},
		{
			name:              "returns error on 404",
			httpCode:          404,
			expectErrContains: "not found on pkg.go.dev",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withHTTPServer("/", func(rw http.ResponseWriter, r *http.Request) {
				if c.httpCode != 0 {
					rw.WriteHeader(c.httpCode)
					return
				}
				rw.Write([]byte(c.html))
			}, func(addr string) {
				client := New(WithBaseURL("http://" + addr))
				pkg, err := client.DescribePackage("somepackage")
				if c.expectErrContains != "" {
					assert.Contains(t, err.Error(), c.expectErrContains)
					return
				}
				assert.NoError(t, err)
				assert.Equal(t, c.expectPackage, *pkg)
			})
		})
	}
}
