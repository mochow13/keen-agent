package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestManagerOAuthDynamicRegistrationIncludesGrantTypes(t *testing.T) {
	var (
		regMu       sync.Mutex
		regBody     map[string]any
		authCode    = "test-auth-code"
		accessTok   = "test-access-token"
		redirectURI = DefaultOAuthRedirectURL
	)

	server := newTestMCPServer()
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return server
	}, nil)

	var httpServer *httptest.Server

	var authServer *httptest.Server
	authServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"resource":                 httpServer.URL,
				"authorization_servers":    []string{authServer.URL},
				"scopes_supported":         []string{"read", "write"},
				"bearer_methods_supported": []string{"header"},
			})

		case "/.well-known/oauth-authorization-server":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                authServer.URL,
				"authorization_endpoint":                authServer.URL + "/authorize",
				"token_endpoint":                        authServer.URL + "/token",
				"registration_endpoint":                 authServer.URL + "/register",
				"jwks_uri":                              authServer.URL + "/jwks",
				"response_types_supported":              []string{"code"},
				"grant_types_supported":                 []string{"authorization_code"},
				"code_challenge_methods_supported":      []string{"S256"},
				"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
			})

		case "/register":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = r.Body.Close()
			regMu.Lock()
			regBody = body
			regMu.Unlock()

			if gt, ok := body["grant_types"]; !ok || gt == nil {
				http.Error(w, `{"error":"invalid_client_metadata","error_description":"missing field grant_types"}`, http.StatusUnprocessableEntity)
				return
			}
			if rt, ok := body["response_types"]; !ok || rt == nil {
				http.Error(w, `{"error":"invalid_client_metadata","error_description":"missing field response_types"}`, http.StatusUnprocessableEntity)
				return
			}
			if am, ok := body["token_endpoint_auth_method"]; !ok || am == nil || am == "" {
				http.Error(w, `{"error":"invalid_client_metadata","error_description":"missing field token_endpoint_auth_method"}`, http.StatusUnprocessableEntity)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id":     "dynamic-client-id",
				"client_secret": "dynamic-client-secret",
			})

		case "/authorize":
			q := r.URL.Query()
			state := q.Get("state")
			redirect := q.Get("redirect_uri")
			if redirect == "" {
				http.Error(w, "missing redirect_uri", http.StatusBadRequest)
				return
			}
			u, err := url.Parse(redirect)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			q = u.Query()
			q.Set("code", authCode)
			q.Set("state", state)
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.String(), http.StatusFound)

		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": accessTok,
				"token_type":   "Bearer",
				"expires_in":   3600,
			})

		case "/jwks":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(authServer.Close)

	metadataURL := authServer.URL + "/.well-known/oauth-protected-resource"
	httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+metadataURL+`"`)
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+accessTok {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(httpServer.Close)

	fetcher := func(ctx context.Context, args *mcpauth.AuthorizationArgs) (*mcpauth.AuthorizationResult, error) {
		u, err := url.Parse(args.URL)
		if err != nil {
			return nil, err
		}
		return &mcpauth.AuthorizationResult{
			Code:  authCode,
			State: u.Query().Get("state"),
		}, nil
	}

	manager := newTestManagerWithOptions(t, map[string]ServerConfig{
		"flog": {
			URL:  httpServer.URL,
			Auth: AuthConfig{Type: AuthOAuth, Scopes: []string{"read", "write"}},
		},
	})

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Startup should be auth_required because no stored token exists and no
	// interactive fetcher is configured by default.
	waitForState(t, manager, "flog", StateAuthRequired)

	// Connect interactively; this exercises dynamic client registration.
	err := manager.Refresh(context.Background(), "flog",
		WithRefreshOAuthRedirectURL(redirectURI),
		WithRefreshOAuthAuthorizationCodeFetcher(fetcher),
		WithRefreshOAuthForceReauth(true),
		WithRefreshConnectTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	waitForState(t, manager, "flog", StateConnected)

	regMu.Lock()
	body := regBody
	regMu.Unlock()
	if body == nil {
		t.Fatal("dynamic registration endpoint was never called")
	}
	if got, ok := body["grant_types"].([]any); !ok || !sliceEqual(got, []any{"authorization_code"}) {
		t.Fatalf("grant_types = %v, want [authorization_code]", body["grant_types"])
	}
	if got, ok := body["response_types"].([]any); !ok || !sliceEqual(got, []any{"code"}) {
		t.Fatalf("response_types = %v, want [code]", body["response_types"])
	}
	if got, want := body["client_name"], "Keen Agent"; got != want {
		t.Fatalf("client_name = %v, want %v", got, want)
	}
	if got, want := body["scope"], "read write"; got != want {
		t.Fatalf("scope = %v, want %v", got, want)
	}
	if got, want := body["token_endpoint_auth_method"], "client_secret_post"; got != want {
		t.Fatalf("token_endpoint_auth_method = %v, want %v", got, want)
	}

	tools, err := manager.ListTools(context.Background(), "flog")
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("ListTools() length = %d, want 2", len(tools))
	}
}

func sliceEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
