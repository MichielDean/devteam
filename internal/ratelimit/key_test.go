package ratelimit

import (
	"net/http"
	"net/url"
	"testing"
)

func TestKeyExtractorRemoteAddr(t *testing.T) {
	// BR-12: RemoteAddr host part, XFF ignored by default.
	ke := KeyExtractor{}
	r := &http.Request{Method: "POST", RemoteAddr: "198.51.100.42:54321"}
	r.URL = &url.URL{Path: "/v1/run"}
	key, ip := ke.Key(r)
	if ip != "198.51.100.42" {
		t.Errorf("ip = %q, want 198.51.100.42", ip)
	}
	wantKey := "198.51.100.42|POST /v1/run"
	if key != wantKey {
		t.Errorf("key = %q, want %q", key, wantKey)
	}
}

func TestKeyExtractorIgnoresXFFByDefault(t *testing.T) {
	// US1 scenario 5: trust_proxy_headers=false + XFF present → uses RemoteAddr.
	ke := KeyExtractor{TrustProxyHeaders: false}
	r := &http.Request{Method: "GET", RemoteAddr: "198.51.100.42:54321"}
	r.URL = &url.URL{Path: "/status"}
	r.Header = http.Header{}
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	_, ip := ke.Key(r)
	if ip != "198.51.100.42" {
		t.Errorf("ip = %q, want 198.51.100.42 (XFF ignored when trust=false)", ip)
	}
}

func TestKeyExtractorMalformedRemoteAddr(t *testing.T) {
	// BR-13: malformed RemoteAddr → fail-safe fallback, no panic.
	ke := KeyExtractor{}
	cases := []string{"garbage", "", "not-an-ip"}
	for _, c := range cases {
		r := &http.Request{Method: "GET", RemoteAddr: c}
		r.URL = &url.URL{Path: "/x"}
		_, ip := ke.Key(r)
		if ip == "" {
			t.Errorf("RemoteAddr=%q: ip empty (expected fallback)", c)
		}
	}
}

func TestKeyExtractorCompositeKey(t *testing.T) {
	// BR-10/BR-16: same IP, different METHOD /path → different keys.
	ke := KeyExtractor{}
	r1 := &http.Request{Method: "POST", RemoteAddr: "1.2.3.4:1"}
	r1.URL = &url.URL{Path: "/v1/run"}
	r2 := &http.Request{Method: "GET", RemoteAddr: "1.2.3.4:1"}
	r2.URL = &url.URL{Path: "/v1/status"}
	k1, _ := ke.Key(r1)
	k2, _ := ke.Key(r2)
	if k1 == k2 {
		t.Errorf("different endpoints produced same key %q", k1)
	}
}

func TestKeyExtractorEndpointString(t *testing.T) {
	// BR-10: endpoint format is exactly "METHOD /path" (no query, no host).
	ke := KeyExtractor{}
	r := &http.Request{Method: "GET", RemoteAddr: "1.2.3.4:1"}
	r.URL = &url.URL{Path: "/v1/status", RawQuery: "foo=bar"}
	key, _ := ke.Key(r)
	// r.URL.Path excludes the query string by definition.
	want := "1.2.3.4|GET /v1/status"
	if key != want {
		t.Errorf("key = %q, want %q (no query string)", key, want)
	}
}

func TestKeyExtractorXFFOptInParsesLeftmost(t *testing.T) {
	// US7 scenario 2, ADR-009: trust=true + XFF "203.0.113.9, 10.0.0.1" → leftmost.
	ke := KeyExtractor{TrustProxyHeaders: true}
	r := &http.Request{Method: "GET", RemoteAddr: "198.51.100.42:54321"}
	r.URL = &url.URL{Path: "/x"}
	r.Header = http.Header{}
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	_, ip := ke.Key(r)
	if ip != "203.0.113.9" {
		t.Errorf("ip = %q, want 203.0.113.9 (leftmost)", ip)
	}
}

func TestKeyExtractorXFFMalformedFallsBack(t *testing.T) {
	// US7 scenario 3: malformed XFF → RemoteAddr + warning logged (no panic).
	ke := KeyExtractor{TrustProxyHeaders: true}
	r := &http.Request{Method: "GET", RemoteAddr: "198.51.100.42:54321"}
	r.URL = &url.URL{Path: "/x"}
	r.Header = http.Header{}
	r.Header.Set("X-Forwarded-For", "not-an-ip")
	_, ip := ke.Key(r)
	if ip != "198.51.100.42" {
		t.Errorf("malformed XFF fallback ip = %q, want 198.51.100.42", ip)
	}
}

func TestKeyExtractorXFFEmptyFallsBack(t *testing.T) {
	// Empty XFF header → RemoteAddr (no warning per NDP-10).
	ke := KeyExtractor{TrustProxyHeaders: true}
	r := &http.Request{Method: "GET", RemoteAddr: "198.51.100.42:54321"}
	r.URL = &url.URL{Path: "/x"}
	r.Header = http.Header{}
	r.Header.Set("X-Forwarded-For", "")
	_, ip := ke.Key(r)
	if ip != "198.51.100.42" {
		t.Errorf("empty XFF fallback ip = %q, want 198.51.100.42", ip)
	}
}

func TestKeyExtractorXFFTrimsWhitespace(t *testing.T) {
	// BR-12/ADR-009: " 203.0.113.9 , 10.0.0.1" → leftmost trimmed = 203.0.113.9.
	ke := KeyExtractor{TrustProxyHeaders: true}
	r := &http.Request{Method: "GET", RemoteAddr: "198.51.100.42:54321"}
	r.URL = &url.URL{Path: "/x"}
	r.Header = http.Header{}
	r.Header.Set("X-Forwarded-For", " 203.0.113.9 , 10.0.0.1")
	_, ip := ke.Key(r)
	if ip != "203.0.113.9" {
		t.Errorf("trimmed leftmost ip = %q, want 203.0.113.9", ip)
	}
}