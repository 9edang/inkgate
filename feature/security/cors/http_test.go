package cors

import (
	"bytes"
	"fmt"
	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/internal/json"
	"net/http"
	"net/http/httptest"
	"regexp"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
)

func ExampleNewRunServerWithLogger() {
	var localHandler http.Handler
	next := func(_ context.Context, _ config.ServiceConfig, handler http.Handler) error {
		localHandler = handler
		return nil
	}

	buf := bytes.NewBuffer(nil)
	l, _ := logging.NewLogger("DEBUG", buf, "")
	corsRunServer := NewRunServerWithLogger(next, l)

	sampleCfg := map[string]any{}
	serialized := []byte(`{ "inkgate/feature/security/cors": {
			"allow_origins": [ "http://foobar.com" ],
			"allow_methods": [ "GET" ],
			"max_age": "2h",
			"debug": true
			}
		}`)
	json.Unmarshal(serialized, &sampleCfg)
	cfg := config.ServiceConfig{ExtraConfig: sampleCfg}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Yeah"))
	})

	if err := corsRunServer(context.Background(), cfg, mux); err != nil {
		fmt.Println(err)
		return
	}

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "http://example.com/", nil)
	req.Header.Add("Origin", "http://foobar.com")
	req.Header.Add("Access-Control-Request-Method", "GET")
	req.Header.Add("Access-Control-Request-Headers", "origin")
	localHandler.ServeHTTP(res, req)
	fmt.Println(res.Code)

	b, _ := json.MarshalIndent(res.Header(), "", "\t")
	fmt.Println(string(b))

	fmt.Println("'" + res.Body.String() + "'")

	res = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "http://example.com/", nil)
	req.Header.Add("Origin", "http://foobar.com")
	req.Header.Add("Access-Control-Request-Method", "GET")
	req.Header.Add("Access-Control-Request-Headers", "origin")
	localHandler.ServeHTTP(res, req)
	fmt.Println(res.Code)

	b, _ = json.MarshalIndent(res.Header(), "", "\t")
	fmt.Println(string(b))

	fmt.Println("'" + res.Body.String() + "'")

	re := regexp.MustCompile(`(\d\d\d\d\/\d\d\/\d\d \d\d:\d\d:\d\d\s+)`)
	fmt.Println(re.ReplaceAllString(buf.String(), ""))

	// output:
	// 204
	// {
	// 	"Access-Control-Allow-Headers": [
	// 		"origin"
	// 	],
	// 	"Access-Control-Allow-Methods": [
	// 		"GET"
	// 	],
	// 	"Access-Control-Allow-Origin": [
	// 		"http://foobar.com"
	// 	],
	// 	"Access-Control-Max-Age": [
	// 		"7200"
	// 	],
	// 	"Vary": [
	// 		"Origin, Access-Control-Request-Method, Access-Control-Request-Headers"
	// 	]
	// }
	// ''
	// 200
	// {
	// 	"Access-Control-Allow-Origin": [
	// 		"http://foobar.com"
	// 	],
	// 	"Content-Type": [
	// 		"text/plain; charset=utf-8"
	// 	],
	// 	"Vary": [
	// 		"Origin"
	// 	]
	// }
	// 'Yeah'
	// DEBUG: [SERVICE: HTTP][CORS] Enabled CORS for all requests
	// DEBUG: [CORS] Handler: Preflight request
	// DEBUG: [CORS] Preflight response headers: map[Access-Control-Allow-Headers:[origin] Access-Control-Allow-Methods:[GET] Access-Control-Allow-Origin:[http://foobar.com] Access-Control-Max-Age:[7200] Vary:[Origin, Access-Control-Request-Method, Access-Control-Request-Headers]]
	// DEBUG: [CORS] Handler: Actual request
	// DEBUG: [CORS] Actual response added headers: map[Access-Control-Allow-Origin:[http://foobar.com] Vary:[Origin]]
}
