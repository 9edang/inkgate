// Package mux is the gorilla/mux renderer for output_encoding "xml".
package mux

import (
	"net/http"

	"github.com/clbanning/mxj/v2"

	"github.com/9edang/inkgate/engine/proxy"
)

func init() {
	mxj.XMLEscapeChars(true)
}

// Render writes the response data as XML.
func Render(w http.ResponseWriter, response *proxy.Response) {
	w.Header().Set("Content-Type", "application/xml")
	if response == nil {
		return
	}
	if err := mxj.Map(response.Data).XmlWriter(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
