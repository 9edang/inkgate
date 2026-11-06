// SPDX-License-Identifier: Apache-2.0

package router

import (
	"io"
	"net/http"

	"github.com/9edang/inkgate/internal/constant"

	"github.com/9edang/inkgate/internal/json"

	"github.com/9edang/inkgate/engine/logging"
)

// DebugHandler creates a dummy handler function, useful for quick integration tests.
func DebugHandler(logger logging.Logger) http.HandlerFunc {
	logPrefixSecondary := "[ENDPOINT /__debug/*]"
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug(logPrefixSecondary, "Method:", r.Method)
		logger.Debug(logPrefixSecondary, "URL:", r.RequestURI)
		logger.Debug(logPrefixSecondary, "Query:", r.URL.Query())
		// logger.Debug(logPrefixSecondary, "Params:", c.Params)
		logger.Debug(logPrefixSecondary, "Headers:", r.Header)
		body, _ := io.ReadAll(r.Body)
		// #nosec G104 -- closing is best effort, the error is not actionable
		r.Body.Close()
		logger.Debug(logPrefixSecondary, "Body:", string(body))

		js, _ := json.Marshal(map[string]string{"message": "pong"})

		w.Header().Set(constant.HeaderContentType, constant.MIMEJSONUTF8)
		// #nosec G104 -- response write is best effort, the client may be gone
		w.Write(js)
	}
}
