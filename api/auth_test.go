package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"code-kanban/utils"
)

func TestResolveAuthRequestSourceUsesForwardedHeadersFromTrustedProxy(t *testing.T) {
	resolved := resolveAuthRequestSource(
		authRequestSourceInput{
			RemoteIP:      "10.0.0.5",
			ForwardedFor:  "203.0.113.9, 10.0.0.5",
			ForwardedHost: "public.example.com, internal.example",
		},
		utils.AuthAccessConfig{
			TrustedProxies: []string{"10.0.0.0/24"},
		},
	)

	if got, want := resolved.SourceIP, "203.0.113.9"; got != want {
		t.Fatalf("SourceIP = %q, want %q", got, want)
	}
	if got, want := resolved.Host, "public.example.com"; got != want {
		t.Fatalf("Host = %q, want %q", got, want)
	}
}

func TestResolveAuthRequestSourceIgnoresForwardedHeadersFromUntrustedProxy(t *testing.T) {
	resolved := resolveAuthRequestSource(
		authRequestSourceInput{
			RemoteIP:      "198.51.100.8",
			ForwardedFor:  "203.0.113.9",
			ForwardedHost: "public.example.com",
		},
		utils.AuthAccessConfig{
			TrustedProxies: []string{"10.0.0.0/24"},
		},
	)

	if got, want := resolved.SourceIP, "198.51.100.8"; got != want {
		t.Fatalf("SourceIP = %q, want %q", got, want)
	}
	if resolved.Host != "" {
		t.Fatalf("Host = %q, want empty", resolved.Host)
	}
}

func TestAuthStatusIgnoresDirectHostBypassRules(t *testing.T) {
	cfg := newAuthTestConfig()
	cfg.Auth.AccessRules.BypassDomains = []string{"public.example.com"}
	app := newAuthRoutesTestApp(cfg)

	resp := mustAuthTestRequest(
		t,
		app,
		http.MethodGet,
		"http://public.example.com/api/v1/auth/status",
		"192.0.2.20:1234",
		nil,
		"",
	)
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	status := decodeAuthStatusItem(t, resp)
	if status.Bypassed {
		t.Fatalf("expected direct host rule to be ignored, got %#v", status)
	}
}

func TestAuthStatusUsesTrustedProxyForwardedHostBypassRules(t *testing.T) {
	cfg := newAuthTestConfig()
	cfg.Auth.AccessRules.BypassDomains = []string{"public.example.com"}
	cfg.Auth.TrustedProxies = []string{"0.0.0.0"}
	app := newAuthRoutesTestApp(cfg)

	resp := mustAuthTestRequest(
		t,
		app,
		http.MethodGet,
		"http://internal.example/api/v1/auth/status",
		"192.0.2.20:1234",
		map[string]string{
			"X-Forwarded-Host": "public.example.com",
		},
		"",
	)
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	status := decodeAuthStatusItem(t, resp)
	if !status.Bypassed {
		t.Fatalf("expected bypassed status, got %#v", status)
	}
	if status.Authenticated {
		t.Fatalf("expected bypass without login, got %#v", status)
	}
}

func TestAuthAccessConfigReadAllowsBypassOnlyRequest(t *testing.T) {
	cfg := newAuthTestConfig()
	cfg.Auth.AccessRules.BypassIPs = []string{"0.0.0.0"}
	app := newAuthRoutesTestApp(cfg)

	resp := mustAuthTestRequest(
		t,
		app,
		http.MethodGet,
		"http://codekanban.test/api/v1/auth/access-config",
		"192.0.2.20:1234",
		nil,
		"",
	)
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestAuthAccessConfigWriteRejectsBypassOnlyRequest(t *testing.T) {
	cfg := newAuthTestConfig()
	cfg.Auth.AccessRules.BypassIPs = []string{"0.0.0.0"}
	app := newAuthRoutesTestApp(cfg)

	resp := mustAuthTestRequest(
		t,
		app,
		http.MethodPost,
		"http://codekanban.test/api/v1/auth/access-config",
		"192.0.2.20:1234",
		nil,
		`{"accessRules":{"bypassIPs":["127.0.0.1"]},"trustedProxies":[]}`,
	)
	if got, want := resp.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestPasswordChangeRejectsBypassOnlyRequest(t *testing.T) {
	cfg := newAuthTestConfig()
	cfg.Auth.AccessRules.BypassIPs = []string{"0.0.0.0"}
	app := newAuthRoutesTestApp(cfg)

	resp := mustAuthTestRequest(
		t,
		app,
		http.MethodPost,
		"http://codekanban.test/api/v1/auth/password/change",
		"192.0.2.20:1234",
		nil,
		`{}`,
	)
	if got, want := resp.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestPasswordDisableRejectsBypassOnlyRequest(t *testing.T) {
	cfg := newAuthTestConfig()
	cfg.Auth.AccessRules.BypassIPs = []string{"0.0.0.0"}
	app := newAuthRoutesTestApp(cfg)

	resp := mustAuthTestRequest(
		t,
		app,
		http.MethodPost,
		"http://codekanban.test/api/v1/auth/password/disable",
		"192.0.2.20:1234",
		nil,
		`{}`,
	)
	if got, want := resp.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func decodeAuthStatusItem(t *testing.T, resp *http.Response) authStatusItem {
	t.Helper()

	var payload struct {
		Body struct {
			Item authStatusItem `json:"item"`
		} `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload.Body.Item
}

func newAuthTestConfig() *utils.AppConfig {
	return &utils.AppConfig{
		DocsPath: "/docs",
		Auth: utils.AuthConfig{
			PasswordHash: "configured",
			TokenSecret:  "secret",
			SessionTTL:   "1h",
			ProxyHeader:  utils.DefaultAuthProxyHeader,
		},
	}
}

func newAuthRoutesTestApp(cfg *utils.AppConfig) *fiber.App {
	app := fiber.New(fiber.Config{Immutable: true})
	registerAuthMiddleware(app, cfg)
	registerAuthRoutes(app, cfg)
	app.Get("/api/v1/protected", func(ctx *fiber.Ctx) error {
		return ctx.SendStatus(http.StatusNoContent)
	})
	return app
}

func mustAuthTestRequest(
	t *testing.T,
	app *fiber.App,
	method string,
	target string,
	remoteAddr string,
	headers map[string]string,
	body string,
) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	if body != "" {
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test %s %s: %v", method, target, err)
	}
	return resp
}
