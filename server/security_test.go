package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectCrossOriginMutations(t *testing.T) {
	paths := []string{
		"/conversations/example/turns",
		"/settings/general",
		"/settings/extensions/add/url",
		"/settings/extensions/add/archive",
		"/settings/extensions/example/remove",
		"/v1/conversations/example/turns",
		"/v1/extensions/register",
		"/v1/extensions/heartbeat",
	}
	headers := []struct {
		name, origin, site string
	}{
		{"foreign origin", "https://untrusted.example", ""},
		{"different local port", "http://127.0.0.1:9000", ""},
		{"opaque origin", "null", ""},
		{"cross-site metadata", "", "cross-site"},
		{"same-site metadata", "", "same-site"},
	}
	for _, h := range headers {
		for _, path := range paths {
			t.Run(h.name+path, func(t *testing.T) {
				stopped := false
				s := New(Options{Token: "secret", StopExtension: func(string) error {
					stopped = true
					return errors.New("handler reached")
				}})
				// Invalid form data keeps an unprotected handler from changing
				// anything while still exercising the real route/middleware.
				req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8743"+path, strings.NewReader("%zz"))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("Origin", h.origin)
				req.Header.Set("Sec-Fetch-Site", h.site)
				req.Header.Set("Authorization", "Bearer secret")
				w := httptest.NewRecorder()
				s.Handler().ServeHTTP(w, req)
				if w.Code != http.StatusForbidden || stopped {
					t.Fatalf("status = %d, stop called = %v; want 403 before handler", w.Code, stopped)
				}
			})
		}
	}
}

func TestAllowLocalBrowserMutation(t *testing.T) {
	for _, site := range []string{"", "same-origin"} {
		t.Run("fetch-site="+site, func(t *testing.T) {
			stopped := false
			s := New(Options{Token: "secret", StopExtension: func(string) error {
				stopped = true
				return errors.New("handler reached")
			}})
			req := httptest.NewRequest(http.MethodPost, "http://localhost:8743/settings/extensions/example/remove", nil)
			req.Header.Set("Origin", "http://localhost:8743")
			req.Header.Set("Sec-Fetch-Site", site)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
			if !stopped || w.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, stop called = %v; want local UI to reach handler without token", w.Code, stopped)
			}
		})
	}
}

func TestLocalHostBoundary(t *testing.T) {
	for _, host := range []string{"untrusted.example:8743", "127.0.0.1.untrusted.example:8743", "192.0.2.1:8743", "0.0.0.0:8743", "[::]:8743"} {
		for _, path := range []string{"/settings", "/static/css/app.css", "/v1/health"} {
			t.Run(host+path, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8743"+path, nil)
				req.Host = host
				// Forwarding headers must not override the actual Host check.
				req.Header.Set("X-Forwarded-Host", "localhost:8743")
				req.Header.Set("Authorization", "Bearer secret")
				w := httptest.NewRecorder()
				New(Options{Token: "secret"}).Handler().ServeHTTP(w, req)
				if w.Code != http.StatusForbidden {
					t.Fatalf("status = %d, want 403", w.Code)
				}
			})
		}
	}
	for _, host := range []string{"localhost", "localhost:8743", "127.0.0.1:8743", "[::1]:8743", "[::1]"} {
		t.Run(host, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8743/settings", nil)
			req.Host = host
			w := httptest.NewRecorder()
			New(Options{Token: "secret"}).Handler().ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
		})
	}
}

func TestExtensionRequestsStillRequireBearer(t *testing.T) {
	for _, token := range []string{"", "wrong", "secret"} {
		t.Run("token="+token, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8743/v1/extensions/register", strings.NewReader(`{"name":"example","callback_url":"http://127.0.0.1:9","channels":[{"id":"example"}]}`))
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			New(Options{Token: "secret"}).Handler().ServeHTTP(w, req)
			want := http.StatusUnauthorized
			if token == "secret" {
				want = http.StatusOK
			}
			if w.Code != want {
				t.Fatalf("status = %d, want %d", w.Code, want)
			}
		})
	}
}

func TestListenRejectsNonLoopback(t *testing.T) {
	for _, addr := range []string{":0", "0.0.0.0:0", "[::]:0", "192.0.2.1:0", "example.invalid:0"} {
		t.Run(addr, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			ready := false
			err := New(Options{Addr: addr}).Listen(ctx, func() { ready = true })
			if err == nil || !strings.Contains(err.Error(), "loopback") || ready {
				t.Fatalf("Listen = %v, ready = %v; want loopback rejection before ready", err, ready)
			}
		})
	}
}

func TestListenAllowsLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:0", "localhost:0"} {
		t.Run(addr, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := false
			err := New(Options{Addr: addr}).Listen(ctx, func() {
				ready = true
				cancel()
			})
			if !ready || !errors.Is(err, context.Canceled) {
				t.Fatalf("Listen = %v, ready = %v; want a running local listener", err, ready)
			}
		})
	}
}
