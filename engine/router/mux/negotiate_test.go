package mux

import "testing"

func TestNegotiateFormat(t *testing.T) {
	offers := []string{"application/json", "text/plain", "application/xml", "application/x-yaml"}
	for accept, want := range map[string]string{
		"":                                 "application/json",
		"application/xml":                  "application/xml",
		"text/html, application/xml;q=0.9": "application/xml",
		"application/x-yaml":               "application/x-yaml",
		"*/*":                              "application/json",
		"image/png":                        "",
	} {
		if got := negotiateFormat(accept, offers...); got != want {
			t.Errorf("Accept %q: got %q, want %q", accept, got, want)
		}
	}
}
