package requestfactory

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
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
			relPath:     "/oc3/daemon/ping",
			expectedUrl: "https://localhost:128/proxy/oc3/daemon/ping",
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
