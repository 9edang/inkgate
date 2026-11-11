package pubsub

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/internal/json"

	"gocloud.dev/pubsub"
	_ "gocloud.dev/pubsub/awssnssqs"
	_ "gocloud.dev/pubsub/azuresb"
	_ "gocloud.dev/pubsub/gcppubsub"
	_ "gocloud.dev/pubsub/kafkapubsub"
	_ "gocloud.dev/pubsub/natspubsub"
	_ "gocloud.dev/pubsub/rabbitpubsub"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
)

var errNoBackendHostDefined = errors.New("no host backend defined")

const (
	publisherNamespace  = "inkgate/feature/connector/pubsub/publisher"
	subscriberNamespace = "inkgate/feature/connector/pubsub/subscriber"
)

func NewBackendFactory(ctx context.Context, logger logging.Logger, bf proxy.BackendFactory) *BackendFactory {
	return &BackendFactory{
		logger: logger,
		bf:     bf,
		ctx:    ctx,
	}
}

type BackendFactory struct {
	ctx    context.Context
	logger logging.Logger
	bf     proxy.BackendFactory
}

func (f *BackendFactory) New(remote *config.Backend) proxy.Proxy {
	if prxy, err := f.initSubscriber(f.ctx, remote); err == nil {
		return prxy
	}

	if prxy, err := f.initPublisher(f.ctx, remote); err == nil {
		return prxy
	}

	return f.bf(remote)
}

func (f *BackendFactory) initPublisher(ctx context.Context, remote *config.Backend) (proxy.Proxy, error) {
	if len(remote.Host) < 1 {
		return proxy.NoopProxy, errNoBackendHostDefined
	}

	dns := remote.Host[0]
	cfg := &publisherCfg{}

	if err := getConfig(remote, publisherNamespace, cfg); err != nil {
		// a backend without this namespace is not a pubsub backend: only a broken config is worth an error
		if _, notFound := errors.AsType[*NamespaceNotFoundErr](err); !notFound {
			f.logger.Error("[BACKEND][PubSub] Error initializing publisher: " + err.Error())
		}

		return proxy.NoopProxy, err
	}

	logPrefix := "[BACKEND: " + dns + cfg.TopicURL + "][PubSub]"
	t, err := pubsub.OpenTopic(ctx, dns+cfg.TopicURL)
	if err != nil {
		f.logger.Error(fmt.Sprintf(logPrefix, err.Error()))
		return proxy.NoopProxy, err
	}

	f.logger.Debug(logPrefix, "Publisher initialized successfully")

	go func() {
		<-ctx.Done()
		// #nosec G104 -- best-effort shutdown, the error is not actionable
		_ = t.Shutdown(context.Background())
	}()

	return func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		headers := map[string]string{}
		for k, vs := range r.Headers {
			headers[k] = vs[0]
		}
		msg := &pubsub.Message{
			Metadata: headers,
			Body:     body,
		}

		if err := t.Send(ctx, msg); err != nil {
			return nil, err
		}

		return &proxy.Response{IsComplete: true}, nil
	}, nil
}

func (f *BackendFactory) initSubscriber(ctx context.Context, remote *config.Backend) (proxy.Proxy, error) {
	if len(remote.Host) < 1 {
		return proxy.NoopProxy, errNoBackendHostDefined
	}

	dns := remote.Host[0]
	cfg := &subscriberCfg{}

	if err := getConfig(remote, subscriberNamespace, cfg); err != nil {
		// a backend without this namespace is not a pubsub backend: only a broken config is worth an error
		if _, notFound := errors.AsType[*NamespaceNotFoundErr](err); !notFound {
			f.logger.Error("[BACKEND][PubSub] Error initializing subscriber: " + err.Error())
		}

		return proxy.NoopProxy, err
	}

	topicURL := dns + cfg.SubscriptionURL
	logPrefix := "[BACKEND: " + topicURL + "][PubSub]"

	sub, err := pubsub.OpenSubscription(ctx, topicURL)
	if err != nil {
		f.logger.Error(fmt.Sprintf(logPrefix, "Error while opening subscription: %s", err.Error()))
		return proxy.NoopProxy, err
	}

	f.logger.Debug(logPrefix, "Subscriber initialized successfully")

	go func() {
		<-ctx.Done()
		// #nosec G104 -- best-effort shutdown, the error is not actionable
		_ = sub.Shutdown(context.Background())
	}()

	ef := proxy.NewEntityFormatter(remote)

	return func(ctx context.Context, _ *proxy.Request) (*proxy.Response, error) {
		msg, err := sub.Receive(ctx)
		if err != nil {
			return nil, err
		}

		var data map[string]any
		if err := remote.Decoder(bytes.NewBuffer(msg.Body), &data); err != nil && !errors.Is(err, io.EOF) {
			// TODO: figure out how to Nack if possible
			// msg.Nack()
			return nil, err
		}

		msg.Ack()

		newResponse := proxy.Response{Data: data, IsComplete: true}
		newResponse = ef.Format(newResponse)

		return &newResponse, nil
	}, nil
}

type publisherCfg struct {
	TopicURL string `json:"topic_url"`
}

type subscriberCfg struct {
	SubscriptionURL string `json:"subscription_url"`
}

func getConfig(remote *config.Backend, namespace string, v any) error {
	cfg, ok := remote.ExtraConfig[namespace]
	if !ok {
		return &NamespaceNotFoundErr{
			Namespace: namespace,
		}
	}

	b, err := json.Marshal(&cfg)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(b, &v); err != nil {
		// sonic's own message spans several lines; keep the log entry on one line
		if failure, ok := json.DecodeError(err); ok {
			return fmt.Errorf("%s: %s", namespace, failure.Message)
		}

		return err
	}

	return nil
}

type NamespaceNotFoundErr struct {
	Namespace string
}

func (n *NamespaceNotFoundErr) Error() string {
	return n.Namespace + " not found in the extra config"
}
