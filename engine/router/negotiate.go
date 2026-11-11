package router

import (
	"net/http"
	"strings"

	"github.com/9edang/inkgate/internal/constant"

	"github.com/goccy/go-yaml"

	"github.com/9edang/inkgate/engine/encoding"
	"github.com/9edang/inkgate/engine/proxy"
)

// YAML is the output_encoding for YAML responses.
const YAML = "yaml"

func init() {
	renderRegister[YAML] = yamlRender
}

func yamlRender(w http.ResponseWriter, response *proxy.Response) {
	w.Header().Set(constant.HeaderContentType, "application/yaml; charset=utf-8")
	var data any = map[string]any{}
	if response != nil {
		data = response.Data
	}
	b, err := yaml.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// #nosec G104 -- response write is best effort, the client may be gone
	w.Write(b)
}

// negotiatedRender picks JSON, XML or YAML from the Accept header, in the same
// order of preference as other inkgate builds (JSON when nothing matches or no Accept).
func negotiatedRender(w http.ResponseWriter, r *http.Request, response *proxy.Response) {
	switch negotiateFormat(r.Header.Get("Accept"), constant.MIMEJSON, constant.MIMEPlain, constant.MIMEXML, "application/x-yaml") {
	case constant.MIMEXML:
		getWithFallback("xml", jsonRender)(w, response)
	case constant.MIMEPlain, "application/x-yaml":
		getWithFallback(YAML, jsonRender)(w, response)
	default:
		getWithFallback(encoding.JSON, jsonRender)(w, response)
	}
}

// negotiateFormat returns the first offer accepted by the Accept header (the common Accept-matching algorithm).
func negotiateFormat(accept string, offered ...string) string {
	var accepted []string
	for part := range strings.SplitSeq(accept, ",") {
		if i := strings.IndexByte(part, ';'); i > 0 {
			part = part[:i]
		}
		if part = strings.TrimSpace(part); part != "" {
			accepted = append(accepted, part)
		}
	}
	if len(accepted) == 0 {
		return offered[0]
	}
	for _, a := range accepted {
		for _, offer := range offered {
			i := 0
			for ; i < len(a) && i < len(offer); i++ {
				if a[i] == '*' || offer[i] == '*' {
					return offer
				}
				if a[i] != offer[i] {
					break
				}
			}
			if i == len(a) {
				return offer
			}
		}
	}

	return ""
}
