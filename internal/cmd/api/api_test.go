package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseField(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		init   map[string]interface{}
		want   map[string]interface{}
	}{
		{
			name: "fields only",
			fields: []string{
				"hello.world=1",
				"hello.monde=2",
				`salut="le monde"`,
			},
			want: map[string]interface{}{
				"hello": map[string]interface{}{
					"world": float64(1),
					"monde": float64(2),
				},
				"salut": "le monde",
			},
		},
		{
			name: "update from fields",
			init: map[string]interface{}{
				"hello": map[string]interface{}{
					"monde": float64(42),
				},
				"salut": "fred",
				"bye":   "ivon",
			},
			fields: []string{
				"hello.world=1",
				"hello.monde=2",
				`salut="le monde"`,
			},
			want: map[string]interface{}{
				"hello": map[string]interface{}{
					"world": float64(1),
					"monde": float64(2),
				},
				"salut": "le monde",
				"bye":   "ivon",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := parseFields(tt.init, tt.fields)
			require.NoError(t, err)
			require.Equal(t, tt.want, out)
		})
	}
}

func TestRedirectCheck(t *testing.T) {
	tests := []struct {
		name                  string
		originalHost          string
		redirectURL           string
		expectUseLastResponse bool
	}{
		{
			name:                  "exact same host",
			originalHost:          "api.example.com",
			redirectURL:           "https://api.example.com/path",
			expectUseLastResponse: false,
		},
		{
			name:                  "same host different port still same hostname",
			originalHost:          "api.example.com",
			redirectURL:           "https://api.example.com:443/path",
			expectUseLastResponse: false,
		},
		{
			name:                  "case-insensitive hostname match",
			originalHost:          "API.Example.Com",
			redirectURL:           "https://api.example.com/path",
			expectUseLastResponse: false,
		},
		{
			name:                  "sibling subdomain is not same host",
			originalHost:          "api.planetscale.com",
			redirectURL:           "https://evil.planetscale.com/path",
			expectUseLastResponse: true,
		},
		{
			name:                  "www sibling of api host",
			originalHost:          "api.example.com",
			redirectURL:           "https://www.example.com/path",
			expectUseLastResponse: true,
		},
		{
			name:                  "different domain",
			originalHost:          "api.example.com",
			redirectURL:           "https://api.another.com/path",
			expectUseLastResponse: true,
		},
		{
			name:                  "localhost to domain",
			originalHost:          "localhost",
			redirectURL:           "https://example.com/path",
			expectUseLastResponse: true,
		},
		{
			name:                  "domain to localhost",
			originalHost:          "example.com",
			redirectURL:           "http://localhost:8080/path",
			expectUseLastResponse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redirectCheck := makeRedirectCheck(tt.originalHost)

			req, err := http.NewRequest("GET", tt.redirectURL, nil)
			require.NoError(t, err)

			err = redirectCheck(req, []*http.Request{})

			if tt.expectUseLastResponse {
				require.Equal(t, http.ErrUseLastResponse, err,
					"Expected ErrUseLastResponse for cross-host redirect")
			} else {
				require.NoError(t, err,
					"Expected nil error for same-host redirect")
			}
		})
	}
}

func TestHandleRedirect(t *testing.T) {
	ctx := context.Background()

	// Cross-host redirects must not carry credentials or user-supplied headers
	// from the original request (Authorization, Cookie, X-Api-Key, etc.).
	credentialHeaders := []string{
		"Authorization",
		"Proxy-Authorization",
		"Cookie",
		"X-Api-Key",
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, name := range credentialHeaders {
			require.Empty(t, r.Header.Get(name),
				"credential header %q must not be present on cross-host redirect request", name)
		}
		// Accept is a common original-request header; it must not be forwarded.
		require.Empty(t, r.Header.Get("Accept"))

		w.Write([]byte("Redirect target content"))
	}))
	defer mockServer.Close()

	redirectRes, err := handleRedirect(ctx, mockServer.URL, false)

	require.NoError(t, err)
	require.NotNil(t, redirectRes)

	body, err := io.ReadAll(redirectRes.Body)
	require.NoError(t, err)
	redirectRes.Body.Close()

	require.Equal(t, "Redirect target content", string(body))
}
