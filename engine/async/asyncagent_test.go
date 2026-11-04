// SPDX-License-Identifier: Apache-2.0

package async

import (
	"github.com/9edang/inkgate/context"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
)

func TestAgentStarter_Start_last(t *testing.T) {
	var firstAgentCalled, secondAgentCalled bool
	firstAgent := func(_ context.Context, opts Options) bool {
		// TODO: check opts
		firstAgentCalled = true
		return false
	}
	secondAgent := func(_ context.Context, opts Options) bool {
		// TODO: check opts
		secondAgentCalled = true
		return true
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan string)
	as := AgentStarter([]Factory{firstAgent, secondAgent})
	agents := []*config.AsyncAgent{
		{},
	}
	wait := as.Start(ctx, agents, logging.NoOp, (chan<- string)(ch), noopProxyFactory)

	if err := wait(); err != nil {
		t.Error(err)
	}

	if !firstAgentCalled {
		t.Error("first agent not called")
	}

	if !secondAgentCalled {
		t.Error("second agent not called")
	}
}

func TestAgentStarter_Start_first(t *testing.T) {
	var firstAgentCalled, secondAgentCalled bool
	firstAgent := func(_ context.Context, opts Options) bool {
		firstAgentCalled = true
		return true
	}
	secondAgent := func(_ context.Context, opts Options) bool {
		secondAgentCalled = true
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan string)
	as := AgentStarter([]Factory{firstAgent, secondAgent})
	agents := []*config.AsyncAgent{
		{},
	}
	wait := as.Start(ctx, agents, logging.NoOp, (chan<- string)(ch), noopProxyFactory)

	if err := wait(); err != nil {
		t.Error(err)
	}

	if !firstAgentCalled {
		t.Error("first agent not called")
	}

	if secondAgentCalled {
		t.Error("second agent called")
	}
}

var noopProxyFactory = proxy.FactoryFunc(func(*config.EndpointConfig) (proxy.Proxy, error) {
	return proxy.NoopProxy, nil
})
