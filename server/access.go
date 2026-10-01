package server

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// access decides which Host values belong to this server, and whether the
// web UI must ask for the startup token.
//
// Loopback with no public origin stays open. A non-loopback listen address
// or a public origin requires a sign-in. Loopback hosts remain allowed so a
// reverse proxy on this machine can connect, but those requests sign in too.
type access struct {
	remote bool
	secure bool
	hosts  map[string]struct{}
}

func parseAccess(addr, rawURL string) (access, error) {
	var bindHost string
	wildcard := false
	if addr != "" {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return access{}, fmt.Errorf("listen address must be host:port")
		}
		bindHost = host
		wildcard = host == "" || host == "0.0.0.0" || host == "::"
		if !wildcard && !loopbackHost(host) && net.ParseIP(host) == nil {
			return access{}, fmt.Errorf("listen address must be an IP, localhost, or a wildcard")
		}
	}

	var urlHost string
	secure := false
	if rawURL != "" {
		host, https, err := originHost(rawURL)
		if err != nil {
			return access{}, err
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
			return access{}, fmt.Errorf("--url must name the origin clients open, not a wildcard address")
		}
		urlHost = host
		secure = https
	}

	if wildcard && (urlHost == "" || loopbackHost(urlHost)) {
		return access{}, fmt.Errorf("a wildcard listen address requires --url set to the public origin clients will open")
	}

	hosts := map[string]struct{}{}
	if urlHost != "" && !loopbackHost(urlHost) {
		hosts[hostKey(urlHost)] = struct{}{}
	}
	if bindHost != "" && !wildcard && !loopbackHost(bindHost) {
		hosts[hostKey(bindHost)] = struct{}{}
	}

	exposed := addr != "" && (wildcard || (bindHost != "" && !loopbackHost(bindHost)))
	remote := exposed || len(hosts) > 0
	return access{remote: remote, secure: secure && remote, hosts: hosts}, nil
}

func originHost(raw string) (string, bool, error) {
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", false, fmt.Errorf("--url must be an http or https origin, like https://golem.example.com")
	}
	return u.Hostname(), u.Scheme == "https", nil
}

func (a access) allows(hostHeader string) bool {
	host := requestHost(hostHeader)
	if loopbackHost(host) {
		return true
	}
	_, ok := a.hosts[hostKey(host)]
	return ok
}

func requestHost(hostHeader string) string {
	host := hostHeader
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host[1 : len(host)-1]
	}
	return host
}

func hostKey(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return strings.ToLower(host)
}

func listenAddr(addr string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("listen address must be host:port")
	}
	// Resolve localhost once and bind that IP. A later lookup must not be
	// able to move the socket off loopback.
	if strings.EqualFold(host, "localhost") {
		resolved, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, err
		}
		if resolved.IP == nil || !resolved.IP.IsLoopback() {
			return nil, fmt.Errorf("localhost must resolve to a loopback IP")
		}
		return net.ListenTCP("tcp", resolved)
	}
	return net.Listen("tcp", addr)
}
