package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
			if got := w.Header().Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
				t.Fatalf("CSP = %q, want local UI protected from framing", got)
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

func TestListenRejectsUnsafeAddress(t *testing.T) {
	cases := []struct {
		addr, rawURL, want string
	}{
		{":0", "", "public origin"},
		{"0.0.0.0:0", "", "public origin"},
		{"[::]:0", "", "public origin"},
		{"0.0.0.0:0", "http://127.0.0.1:8743", "public origin"},
		{"0.0.0.0:0", "http://0.0.0.0:8743", "wildcard"},
		{"example.invalid:0", "", "IP"},
		{"127.0.0.1:8743", "https://golem.example.com/extra", "origin"},
		{"0.0.0.0:8743", "http://192.0.2.1:8743", "token"},
	}
	for _, tc := range cases {
		t.Run(tc.addr+" "+tc.rawURL, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			ready := false
			err := New(Options{Addr: tc.addr, URL: tc.rawURL}).Listen(ctx, func() { ready = true })
			if err == nil || !strings.Contains(err.Error(), tc.want) || ready {
				t.Fatalf("Listen = %v, ready = %v; want error containing %q before ready", err, ready, tc.want)
			}
		})
	}
}

func TestListenAllowsPublicOrigin(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:0", "0.0.0.0:0"} {
		t.Run(addr, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := false
			err := New(Options{Addr: addr, URL: "http://192.0.2.1:8743", Token: "secret"}).Listen(ctx, func() {
				ready = true
				cancel()
			})
			if !ready || !errors.Is(err, context.Canceled) {
				t.Fatalf("Listen = %v, ready = %v; want a listener for an explicit public origin", err, ready)
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

func TestPublicHostRequiresSignIn(t *testing.T) {
	h := New(Options{Addr: "192.0.2.1:8743", Token: "secret"}).Handler()
	for _, host := range []string{"192.0.2.1:8743", "127.0.0.1:8743"} {
		t.Run(host, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+host+"/settings", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != http.StatusSeeOther || locationPath(t, w) != "/login" {
				t.Fatalf("status = %d, location = %q; want 303 /login", w.Code, w.Header().Get("Location"))
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "http://192.0.2.1:8743/settings", nil)
	req.Host = "198.51.100.1:8743"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for an unlisted host", w.Code)
	}

	cookie := signIn(t, h, "http://192.0.2.1:8743")
	if cookie.Secure {
		t.Fatal("Secure cookie on an http origin would not be stored by the browser")
	}
}

func TestRemoteSignIn(t *testing.T) {
	const origin = "https://golem.example.com"
	s := New(Options{Addr: "127.0.0.1:8743", URL: origin, Token: "secret"})
	h := s.Handler()

	req := httptest.NewRequest(http.MethodGet, origin+"/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `type="password"`) {
		t.Fatalf("login status = %d, body = %q; want the sign-in form", w.Code, w.Body)
	}
	if got := w.Header().Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
		t.Fatalf("CSP = %q", got)
	}

	css := httptest.NewRequest(http.MethodGet, origin+"/static/css/app.css", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, css)
	if w.Code != http.StatusOK {
		t.Fatalf("static status = %d, want 200 without a session", w.Code)
	}

	bad := httptest.NewRequest(http.MethodPost, origin+"/login", strings.NewReader("token=nope"))
	bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	bad.Header.Set("Origin", origin)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, bad)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "The token is incorrect.") || len(w.Result().Cookies()) != 0 {
		t.Fatalf("status = %d, cookies = %d, body = %q", w.Code, len(w.Result().Cookies()), w.Body)
	}

	foreign := httptest.NewRequest(http.MethodPost, origin+"/login", strings.NewReader("token=secret"))
	foreign.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	foreign.Header.Set("Origin", "https://untrusted.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, foreign)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin login status = %d, want 403", w.Code)
	}

	cookie := signIn(t, h, origin)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || !cookie.Secure || cookie.MaxAge != int(sessionLifetime.Seconds()) {
		t.Fatalf("cookie = %#v, want HttpOnly SameSite=Strict Secure session", cookie)
	}

	settings := httptest.NewRequest(http.MethodGet, origin+"/settings", nil)
	settings.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, settings)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Log out") {
		t.Fatalf("settings status = %d, body = %q; want the signed-in page", w.Code, w.Body)
	}

	stopped := false
	s.opts.StopExtension = func(string) error {
		stopped = true
		return errors.New("handler reached")
	}
	h = s.Handler()
	remove := httptest.NewRequest(http.MethodPost, origin+"/settings/extensions/example/remove", nil)
	remove.Header.Set("Origin", origin)
	remove.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, remove)
	if !stopped || w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, stop called = %v; want signed-in same-origin mutation to reach the handler", w.Code, stopped)
	}

	stopped = false
	cross := httptest.NewRequest(http.MethodPost, origin+"/settings/extensions/example/remove", nil)
	cross.Header.Set("Origin", "https://untrusted.example")
	cross.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, cross)
	if w.Code != http.StatusForbidden || stopped {
		t.Fatalf("status = %d, stop called = %v; want cross-origin mutation rejected with a session", w.Code, stopped)
	}

	apiReq := httptest.NewRequest(http.MethodPost, origin+"/v1/extensions/register", strings.NewReader(`{"name":"example","callback_url":"http://127.0.0.1:9","channels":[{"id":"example"}]}`))
	apiReq.Header.Set("Content-Type", "application/json")
	apiReq.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, apiReq)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("API status = %d, want 401 when only the session cookie is present", w.Code)
	}
	apiReq.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, apiReq)
	if w.Code != http.StatusOK {
		t.Fatalf("API status = %d, want 200 with the bearer token and no reliance on the cookie", w.Code)
	}

	out := httptest.NewRequest(http.MethodPost, origin+"/logout", nil)
	out.Header.Set("Origin", origin)
	out.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, out)
	if w.Code != http.StatusSeeOther || locationPath(t, w) != "/login" {
		t.Fatalf("logout status = %d, location = %q", w.Code, w.Header().Get("Location"))
	}
	cleared := w.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatalf("logout cookie = %v, want a cleared session", cleared)
	}
	again := httptest.NewRequest(http.MethodGet, origin+"/settings", nil)
	again.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, again)
	if w.Code != http.StatusSeeOther || locationPath(t, w) != "/login" {
		t.Fatalf("after logout status = %d, location = %q", w.Code, w.Header().Get("Location"))
	}
}

func TestLocalUIHasNoSignIn(t *testing.T) {
	h := New(Options{Token: "secret"}).Handler()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8743/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther || locationPath(t, w) != "/" {
		t.Fatalf("status = %d, location = %q; want local /login to return home", w.Code, w.Header().Get("Location"))
	}
	home := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8743/", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, home)
	if w.Code != http.StatusOK {
		t.Fatalf("home status = %d, want 200 without a session", w.Code)
	}
}

func signIn(t *testing.T, h http.Handler, origin string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, origin+"/login", strings.NewReader("token=secret"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther || locationPath(t, w) != "/" {
		t.Fatalf("login status = %d, location = %q, body = %s", w.Code, w.Header().Get("Location"), w.Body)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookie || cookies[0].Value == "" {
		t.Fatalf("cookies = %v", cookies)
	}
	return cookies[0]
}

func locationPath(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return u.Path
}
