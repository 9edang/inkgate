//go:build integration || !race
// +build integration !race

// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"net/http"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy/plugin"
)

func TestNewPluginMiddleware_logger(t *testing.T) {
	plugin.LoadWithLogger("./plugin/tests", ".so", plugin.RegisterModifier, logging.NoOp)

	validator := func(ctx context.Context, r *Request) (*Response, error) {
		if r.Path != "/bar/fooo/fooo" {
			return nil, fmt.Errorf("unexpected path %s", r.Path)
		}
		return &Response{
			Data:       map[string]any{"foo": "bar"},
			IsComplete: true,
			Metadata: Metadata{
				Headers:    map[string][]string{},
				StatusCode: 0,
			},
		}, nil
	}

	bknd := NewBackendPluginMiddleware(
		logging.NoOp,
		&config.Backend{
			ExtraConfig: map[string]any{
				plugin.Namespace: map[string]any{
					"name": []any{"inkgate-request-modifier-example-request"},
				},
			},
		},
	)(validator)

	p := NewPluginMiddleware(
		logging.NoOp,
		&config.EndpointConfig{
			ExtraConfig: map[string]any{
				plugin.Namespace: map[string]any{
					"name": []any{
						"inkgate-request-modifier-example-request",
						"inkgate-request-modifier-example-response",
					},
				},
			},
		},
	)(bknd)

	resp, err := p(context.Background(), &Request{Path: "/bar"})
	if err != nil {
		t.Error(err.Error())
	}

	if resp == nil {
		t.Errorf("unexpected response: %v", resp)
		return
	}

	if v, ok := resp.Data["foo"].(string); !ok || v != "bar" {
		t.Errorf("unexpected foo value: %v", resp.Data["foo"])
	}
}

func TestNewPluginMiddleware_error_request(t *testing.T) {
	plugin.LoadWithLogger("./plugin/tests", ".so", plugin.RegisterModifier, logging.NoOp)

	validator := func(ctx context.Context, r *Request) (*Response, error) {
		t.Error("the backend should not be called")
		return nil, nil
	}

	bknd := NewBackendPluginMiddleware(
		logging.NoOp,
		&config.Backend{},
	)(validator)

	p := NewPluginMiddleware(
		logging.NoOp,
		&config.EndpointConfig{
			ExtraConfig: map[string]any{
				plugin.Namespace: map[string]any{
					"name": []any{
						"inkgate-error-example-request",
					},
				},
			},
		},
	)(bknd)

	resp, err := p(context.Background(), &Request{Path: "/bar"})

	if resp != nil {
		t.Errorf("unexpected response: %v", resp)
		return
	}

	if err == nil {
		t.Error("error expected")
		return
	}

	customErr, ok := err.(statusCodeError)

	if !ok {
		t.Errorf("unexpected error: %+v (%T)", err, err)
		return
	}

	if sc := customErr.StatusCode(); sc != http.StatusTeapot {
		t.Errorf("unexpected status code: %d", sc)
	}

	if errorMsg := err.Error(); errorMsg != "request rejected just because" {
		t.Errorf("unexpected error message. have: '%s'", errorMsg)
	}
}

func TestNewPluginMiddleware_error_response(t *testing.T) {
	plugin.LoadWithLogger("./plugin/tests", ".so", plugin.RegisterModifier, logging.NoOp)

	var hit bool

	validator := func(ctx context.Context, r *Request) (*Response, error) {
		hit = true
		return &Response{
			Data:       map[string]any{"foo": "bar"},
			IsComplete: true,
			Metadata: Metadata{
				Headers: map[string][]string{},
			},
		}, nil
	}

	bknd := NewBackendPluginMiddleware(
		logging.NoOp,
		&config.Backend{},
	)(validator)

	p := NewPluginMiddleware(
		logging.NoOp,
		&config.EndpointConfig{
			ExtraConfig: map[string]any{
				plugin.Namespace: map[string]any{
					"name": []any{
						"inkgate-error-example-response",
					},
				},
			},
		},
	)(bknd)

	resp, err := p(context.Background(), &Request{Path: "/bar"})

	if resp != nil {
		t.Errorf("unexpected response: %v", resp)
		return
	}

	if err == nil {
		t.Error("error expected")
		return
	}

	customErr, ok := err.(statusCodeError)

	if !ok {
		t.Errorf("unexpected error: %+v (%T)", err, err)
		return
	}

	if sc := customErr.StatusCode(); sc != http.StatusTeapot {
		t.Errorf("unexpected status code: %d", sc)
	}

	if errorMsg := err.Error(); errorMsg != "response replaced because reasons" {
		t.Errorf("unexpected error message. have: '%s'", errorMsg)
	}

	if !hit {
		t.Error("the backend has not been called")
	}
}

func TestNewPluginMiddleware_PoisonedPlugin(t *testing.T) {
	plugin.RegisterModifier("poisoned", func(map[string]any) func(any) (any, error) {
		return nil
	}, false, true)

	expectedResp := &Response{
		Data:       map[string]any{"foo": "bar"},
		IsComplete: true,
		Metadata: Metadata{
			Headers: map[string][]string{},
		},
	}

	validator := func(ctx context.Context, r *Request) (*Response, error) {
		return expectedResp, nil
	}

	bknd := NewBackendPluginMiddleware(
		logging.NoOp,
		&config.Backend{},
	)(validator)

	p := NewPluginMiddleware(
		logging.NoOp,
		&config.EndpointConfig{
			ExtraConfig: map[string]any{
				plugin.Namespace: map[string]any{
					"name": []any{
						"poisoned",
					},
				},
			},
		},
	)(bknd)

	resp, err := p(context.Background(), &Request{Path: "/bar"})

	if resp != expectedResp {
		t.Errorf("unexpected response: %v", resp)
		return
	}

	if err != nil {
		t.Error("error expected")
		return
	}
}

type statusCodeError interface {
	error
	StatusCode() int
}
