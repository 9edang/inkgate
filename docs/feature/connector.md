# feature/connector — backends that are not HTTP

**Code:** [feature/connector](../../feature/connector): `amqp`, `pubsub`, `lambda`

A connector is a stage-5 step that **makes the backend call itself** when its namespace is in the backend's
`extra_config`. The steps after it (Martian, the HTTP client) never run for that backend.

```mermaid
flowchart LR
    S[stage 5 steps 1-7] --> L{lambda ns?}
    L -- yes --> AWS[(AWS Lambda<br/>invoke function)]
    L -- no --> A{amqp ns?}
    A -- produce --> MQP[(RabbitMQ publish)]
    A -- consume --> MQC[(RabbitMQ get one message)]
    A -- no --> P{pubsub ns?}
    P -- publisher --> TP[(topic publish)]
    P -- subscriber --> TS[(subscription receive)]
    P -- no --> HTTP[Martian → HTTP client]
```

## amqp — RabbitMQ

| Alias | Namespace | Use |
|---|---|---|
| `backend/amqp/producer` | `inkgate/feature/connector/amqp/produce` | publish the request body as a message |
| `backend/amqp/consumer` | `inkgate/feature/connector/amqp/consume` | read a message and return it as the response |
| `async/amqp` | `inkgate/feature/connector/amqp/agent` | async agent: consume continuously and run a pipeline per message (see [../engine/async-and-plugins.md](../engine/async-and-plugins.md)) |

The backend `host` is the AMQP URL (`amqp://user:pass@host:5672`).

**Producer options:** `name` (exchange), `exchange`, `routing_key` (from a param) or `static_routing_key`,
`durable`, `delete`, `exclusive`, `no_wait`, `mandatory`, `immediate`, `exp_key`, `reply_to_key`, `msg_id_key`,
`priority_key` (request headers to copy into message properties), `max_retries`, `backoff_strategy`.

**Consumer options:** `name` (queue), `exchange`, `routing_key`, `durable`, `delete`, `exclusive`, `no_wait`,
`no_local`, `auto_ack`, `nack_discard`, `prefetch_count`, `prefetch_size`, `max_retries`, `backoff_strategy`.

## pubsub — Go CDK pub/sub

| Alias | Namespace | Options |
|---|---|---|
| `backend/pubsub/publisher` | `inkgate/feature/connector/pubsub/publisher` | `topic_url` |
| `backend/pubsub/subscriber` | `inkgate/feature/connector/pubsub/subscriber` | `subscription_url` |

The URL scheme picks the provider: `gcppubsub://`, `awssns://` / `awssqs://`, `azuresb://`, `kafka://`, `nats://`,
`rabbit://`, `mem://`. Credentials come from each provider's usual environment variables.

## lambda — AWS Lambda

| Namespace | `inkgate/feature/connector/lambda` (alias `backend/lambda`) |
|---|---|
| Options | `function_name` (fixed) or `function_param_name` (taken from a request param), `region`, `endpoint` (custom, e.g. LocalStack), `max_retries` |

For `GET` backends the request params are the payload; for other methods it's the request body. The function's result payload is decoded as JSON and becomes the backend response.
