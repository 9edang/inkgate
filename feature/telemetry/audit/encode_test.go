package audit

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/proxy/plugin"
	router "github.com/9edang/inkgate/engine/router"
	client "github.com/9edang/inkgate/engine/transport/http/client/plugin"
	server "github.com/9edang/inkgate/engine/transport/http/server/plugin"
	bf "github.com/9edang/inkgate/feature/auth/bloomfilter/gateway"
	botdetector "github.com/9edang/inkgate/feature/security/botdetector/gateway"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
	ratelimit "github.com/9edang/inkgate/feature/traffic/ratelimit/router"
)

func intn(k int) int {
	n, err := rand.Read(nil)
	if err != nil {
		return time.Now().Nanosecond() % k
	}
	return n % k
}

func TestMarshal(t *testing.T) {
	result := Parse(generateCfg())

	b, err := Marshal(&result)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("Compressed (best):", len(b))

	var out Service
	if err := Unmarshal(b, &out); err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(result, out) {
		t.Error("sent and received are different")
	}
}

func generateCfg() *config.ServiceConfig {
	cfg := &config.ServiceConfig{
		SequentialStart: true,
		Endpoints:       []*config.EndpointConfig{},
		ExtraConfig: config.ExtraConfig{
			server.Namespace:      map[string]any{},
			router.Namespace:      map[string]any{},
			bf.Namespace:          map[string]any{},
			botdetector.Namespace: map[string]any{},
			opencensus.Namespace:  map[string]any{},
			ratelimit.Namespace:   map[string]any{},
		},
		AsyncAgents: []*config.AsyncAgent{
			{
				Name:       "first",
				Encoding:   "json",
				Consumer:   config.Consumer{},
				Connection: config.Connection{},
				ExtraConfig: config.ExtraConfig{
					"component3":        true,
					ratelimit.Namespace: map[string]any{},
				},
				Backend: []*config.Backend{
					{
						URLPattern: "/foo",
						Group:      "bar",
						AllowList:  []string{"foo", "bar"},
						Mapping:    map[string]string{"foo": "foobar"},
						Encoding:   "json",
						ExtraConfig: config.ExtraConfig{
							"component4":        true,
							"component5":        true,
							ratelimit.Namespace: map[string]any{},
						},
					},
				},
			},
		},
		Plugin: &config.Plugin{
			Folder: ".",
		},
		TLS: &config.TLS{
			PublicKey:  "./cert.pub",
			PrivateKey: "./cert.key",
		},
	}

	for range 1000 {
		var endpoints []*config.EndpointConfig

		for range 1 + intn(3) {
			e := &config.EndpointConfig{
				Endpoint:       fmt.Sprintf("/foo/%3d", intn(100)),
				OutputEncoding: "json",
				Backend:        []*config.Backend{},
				ExtraConfig: config.ExtraConfig{
					fmt.Sprintf("component%3d", intn(100)): true,
					fmt.Sprintf("component%3d", intn(100)): true,
					plugin.Namespace:                       map[string]any{},
					proxy.Namespace:                        map[string]any{},
				},
			}
			for j := range 1 + intn(5) {
				e.Backend = append(e.Backend, &config.Backend{
					URLPattern: fmt.Sprintf("/foo/%3d", intn(100)),
					Group:      fmt.Sprintf("group-%3d", j),
					AllowList:  []string{"foo", "bar"},
					Mapping:    map[string]string{"foo": "foobar"},
					Encoding:   "json",
					ExtraConfig: config.ExtraConfig{
						fmt.Sprintf("component%3d", intn(100)): true,
						fmt.Sprintf("component%3d", intn(100)): true,
						client.Namespace:                       map[string]any{},
					},
				})
			}
			endpoints = append(endpoints, e)
		}
		for range 1 + intn(3) {
			e := &config.EndpointConfig{
				Endpoint:       fmt.Sprintf("/foo/%3d", intn(100)),
				OutputEncoding: "no-op",
				Backend: []*config.Backend{
					{
						URLPattern: fmt.Sprintf("/foo/%3d", intn(100)),
						Group:      "first",
						AllowList:  []string{"foo", "bar"},
						Mapping:    map[string]string{"foo": "foobar"},
						Encoding:   "no-op",
						ExtraConfig: config.ExtraConfig{
							fmt.Sprintf("component%3d", intn(100)): true,
							client.Namespace:                       map[string]any{},
						},
					},
				},
				ExtraConfig: config.ExtraConfig{
					fmt.Sprintf("component%3d", intn(100)): true,
					fmt.Sprintf("component%3d", intn(100)): true,
					plugin.Namespace:                       map[string]any{},
					proxy.Namespace:                        map[string]any{},
				},
			}
			endpoints = append(endpoints, e)
		}

		cfg.Endpoints = append(cfg.Endpoints, endpoints...)
	}

	return cfg
}
