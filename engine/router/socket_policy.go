// SPDX-License-Identifier: Apache-2.0

package router

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// originPolicy is the allowed_origins list of a socket endpoint. A browser always sends Origin with a WebSocket
// upgrade and, unlike fetch, does not apply CORS to it: without a check here any page the user visits can open a socket
// with the user's cookies (cross-site WebSocket hijacking). The zero value allows every origin; an empty list allows
// none (only clients that send no Origin).
type originPolicy struct {
	on       bool // the list is configured
	any      bool
	exact    map[string]bool // "https://app.example.com" (normalized)
	suffixes []originSuffix  // "https://*.example.com"
}

// originSuffix matches every subdomain of host (not host itself) with the same scheme and port.
type originSuffix struct {
	scheme, suffix, port string // suffix is ".example.com"
}

func (p originPolicy) enabled() bool { return p.on }

// allows tells if a request with this Origin header may open a socket. No Origin means the client is not a browser:
// the attack needs a browser, and a non-browser client can send any Origin it wants anyway.
func (p originPolicy) allows(origin string) bool {
	if !p.enabled() || p.any || origin == "" {
		return true
	}
	o, ok := splitOrigin(origin)
	if !ok {
		return false
	}
	if p.exact[o.String()] {
		return true
	}
	for _, s := range p.suffixes {
		if s.scheme == o.scheme && s.port == o.port && strings.HasSuffix(o.host, s.suffix) {
			return true
		}
	}

	return false
}

// webOrigin is scheme://host:port, lower case, with the port always set.
type webOrigin struct{ scheme, host, port string }

func (o webOrigin) String() string { return o.scheme + "://" + net.JoinHostPort(o.host, o.port) }

// splitOrigin reads scheme://host[:port], with the default port of the scheme filled in. A path, query, user or
// anything else than an origin is refused.
func splitOrigin(s string) (webOrigin, bool) {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return webOrigin{}, false
	}
	o := webOrigin{scheme: strings.ToLower(u.Scheme), host: strings.ToLower(u.Hostname()), port: u.Port()}
	if o.port == "" {
		switch o.scheme {
		case "http", "ws":
			o.port = "80"
		case "https", "wss":
			o.port = "443"
		default:
			return webOrigin{}, false
		}
	}

	return o, o.host != ""
}

func parseOriginPolicy(list []string) (originPolicy, error) {
	p := originPolicy{on: true, exact: map[string]bool{}}
	for _, o := range list {
		if o == "*" {
			p.any = true
			continue
		}
		// "https://*.example.com" is read as "https://example.com" and matches its subdomains
		s, wild := o, false
		if sch, rest, found := strings.Cut(o, "://*."); found {
			s, wild = sch+"://"+rest, true
		}
		so, ok := splitOrigin(s)
		if !ok || strings.Contains(so.host, "*") {
			return originPolicy{}, fmt.Errorf("%q is not an origin like \"https://app.example.com\" or \"https://*.example.com\"", o)
		}
		if wild {
			p.suffixes = append(p.suffixes, originSuffix{scheme: so.scheme, suffix: "." + so.host, port: so.port})
			continue
		}
		p.exact[so.String()] = true
	}

	return p, nil
}

// subprotocolPolicy is the subprotocols list of a socket endpoint: only those reach the backend. Nil allows all.
type subprotocolPolicy []string

// filter keeps the offered subprotocols that are allowed, in the client's order (its preference). ok is false when the
// client offered some and none is allowed: no backend could answer with one the client accepts.
func (p subprotocolPolicy) filter(offered []string) (kept []string, ok bool) {
	if p == nil || len(offered) == 0 {
		return offered, true
	}
	for _, o := range offered {
		for _, a := range p {
			if o == a { // subprotocol names are case-sensitive (RFC 6455 4.1)
				kept = append(kept, o)
				break
			}
		}
	}

	return kept, len(kept) > 0
}

// stringList reads a JSON array of non-empty strings.
func stringList(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		for _, s := range v {
			if s == "" {
				return nil, false
			}
		}

		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			s, ok := e.(string)
			if !ok || s == "" {
				return nil, false
			}
			out = append(out, s)
		}

		return out, true
	}

	return nil, false
}
