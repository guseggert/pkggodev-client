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
		name            string
		html            string
		httpErr         bool
		expectedImports []string
		expectedErr     string
	}{
		{

			name: "happy case",
			html: `
<html><body>
<div class="u-breakWord">foo</div>
<div class="u-breakWord">bar</div>
</body></html>
`,
			expectedImports: []string{"foo", "bar"},
		},
		{
			name:            "no results",
			html:            "",
			expectedImports: []string{},
		},
		{
			name:        "http error returns an error",
			html:        "",
			httpErr:     true,
			expectedErr: "Internal Server Error",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withHTTPServer("/", func(rw http.ResponseWriter, r *http.Request) {
				if c.httpErr {
					rw.WriteHeader(500)
					return
				}
				rw.Write([]byte(c.html))
			}, func(addr string) {
				client := New(WithBaseURL("http://" + addr))
				imps, err := client.ImportedBy("somepackage")
				if c.expectedErr != "" {
					assert.Contains(t, err.Error(), c.expectedErr)
					return
				}
				assert.NoError(t, err)
				assert.Equal(t, c.expectedImports, imps)
			})
		})
	}
}
