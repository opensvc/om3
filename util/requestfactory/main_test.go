package requestfactory

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/oc3path"
)

func TestNew(t *testing.T) {
	baseUrl, err := url.Parse("https://localhost:128/proxy")
	require.NoError(t, err)
	defaultHeaders := http.Header{
		"Authorization": []string{"Basic baz"},
	}
	factory := New(baseUrl, defaultHeaders)

	cases := []struct {
		method      string
		relPath     string
		expectedUrl string
	}{
		{
			method:      "POST",
			relPath:     oc3path.FeedDaemonPing,
			expectedUrl: fmt.Sprintf("https://localhost:128/proxy%s", oc3path.FeedDaemonPing),
		},
		{
			method:      "GET",
			relPath:     "foo",
			expectedUrl: "https://localhost:128/proxy/foo",
		},
	}

	for _, c := range cases {
		t.Run("create request "+c.method+" "+c.relPath, func(t *testing.T) {
			req, err := factory.NewRequest(c.method, c.relPath, nil)
			require.NoError(t, err, "can't create request")
			require.Equal(t, c.expectedUrl, req.URL.String())
			require.Equal(t, c.method, req.Method)
			require.Equal(t, "Basic baz", req.Header.Get("Authorization"))
		})
	}
}
