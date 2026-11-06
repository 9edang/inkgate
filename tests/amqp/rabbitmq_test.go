// Package amqp_test runs the AMQP connector against a real RabbitMQ started with testcontainers-go.
// The tests skip themselves when no container runtime is reachable (set DOCKER_HOST for podman).
package amqp_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

const rabbitmqImage = "rabbitmq:3-management"

var (
	startOnce sync.Once
	container *rabbitmq.RabbitMQContainer
	amqpURL   string
	startErr  error
)

// rabbitmqURL starts one RabbitMQ for the whole package, on first use, and returns its AMQP URL
// (amqp://guest:guest@host:port). The container is stopped by TestMain.
func rabbitmqURL(t *testing.T) string {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)

	startOnce.Do(func() {
		container, startErr = rabbitmq.Run(context.Background(), rabbitmqImage)
		if startErr != nil {
			return
		}
		amqpURL, startErr = container.AmqpURL(context.Background())
	})
	if startErr != nil {
		t.Fatalf("cannot start RabbitMQ: %s", startErr)
	}

	return amqpURL
}

func TestMain(m *testing.M) {
	code := m.Run()
	if container != nil {
		_ = testcontainers.TerminateContainer(container)
	}
	os.Exit(code)
}
