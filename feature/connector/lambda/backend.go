package lambda

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/internal/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
)

const (
	Namespace = "inkgate/feature/connector/lambda"
)

var (
	errBadStatusCode = errors.New("aws lambda: bad status code")
	errNoConfig      = errors.New("aws lambda: no extra config defined")
	errBadConfig     = errors.New("aws lambda: unable to parse the defined extra config")
)

// Invoker is the part of the AWS Lambda client the backend uses.
type Invoker interface {
	Invoke(context.Context, *lambda.InvokeInput, ...func(*lambda.Options)) (*lambda.InvokeOutput, error)
}

func BackendFactory(logger logging.Logger, bf proxy.BackendFactory) proxy.BackendFactory {
	return BackendFactoryWithInvoker(logger, bf, invokerFactory)
}

// invokerFactory builds the Lambda client. Credentials and, unless the config sets it, the region come from the
// default AWS chain (environment, shared files, instance role).
func invokerFactory(o *Options) Invoker {
	var loadOpts []func(*awsconfig.LoadOptions) error
	if o.Region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(o.Region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return failedInvoker{err: fmt.Errorf("aws lambda: loading the AWS configuration: %w", err)}
	}

	return lambda.NewFromConfig(cfg, func(lo *lambda.Options) {
		if o.Endpoint != "" {
			lo.BaseEndpoint = aws.String(o.Endpoint)
		}
		if o.MaxRetries != nil {
			lo.RetryMaxAttempts = *o.MaxRetries + 1 // the first call is not a retry
		}
	})
}

// failedInvoker reports a client that could not be built at every call, so the backend fails on its own requests
// instead of the gateway at startup.
type failedInvoker struct{ err error }

func (f failedInvoker) Invoke(context.Context, *lambda.InvokeInput, ...func(*lambda.Options)) (*lambda.InvokeOutput, error) {
	return nil, f.err
}

func BackendFactoryWithInvoker(logger logging.Logger, bf proxy.BackendFactory, invokerFactory func(*Options) Invoker) proxy.BackendFactory {
	return func(remote *config.Backend) proxy.Proxy {
		logPrefix := "[BACKEND: " + remote.URLPattern + "][Lambda]"
		ecfg, err := getOptions(remote)
		if err != nil {
			if !errors.Is(err, errNoConfig) {
				logger.Error(logPrefix, err)
			}

			return bf(remote)
		}

		i := invokerFactory(ecfg)

		ef := proxy.NewEntityFormatter(remote)

		logger.Debug(logPrefix, "Component enabled")

		return func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
			payload, err := ecfg.PayloadExtractor(r)
			if err != nil {
				return nil, err
			}
			input := &lambda.InvokeInput{
				// ClientContext:  aws.String(clientContext),
				FunctionName:   aws.String(ecfg.FunctionExtractor(r)),
				InvocationType: types.InvocationTypeRequestResponse,
				LogType:        types.LogTypeNone,
				Payload:        payload,
				// Qualifier:      aws.String("1"),
			}

			result, err := i.Invoke(ctx, input)
			if err != nil {
				return nil, err
			}
			if result.StatusCode != http.StatusOK {
				return nil, errBadStatusCode
			}

			data := map[string]any{}
			if err := json.Unmarshal(result.Payload, &data); err != nil {
				return nil, err
			}
			response := ef.Format(proxy.Response{
				Metadata: proxy.Metadata{
					StatusCode: int(result.StatusCode),
					Headers:    map[string][]string{},
				},
				Data:       data,
				IsComplete: true,
			})

			if result.ExecutedVersion != nil {
				response.Metadata.Headers["X-Amz-Executed-Version"] = []string{*result.ExecutedVersion}
			}

			return &response, nil
		}
	}
}

func getOptions(remote *config.Backend) (*Options, error) {
	v, ok := remote.ExtraConfig[Namespace]
	if !ok {
		return nil, errNoConfig
	}
	ecfg, ok := v.(map[string]any)
	if !ok {
		return nil, errBadConfig
	}

	var funcExtractor functionExtractor
	funcName, ok := ecfg["function_name"].(string)
	if ok {
		funcExtractor = func(_ *proxy.Request) string {
			return funcName
		}
	} else {
		funcParamName, ok := ecfg["function_param_name"].(string)
		if !ok {
			funcParamName = "function"
		}
		funcExtractor = func(r *proxy.Request) string {
			return r.Params[funcParamName]
		}
	}

	cfg := &Options{
		FunctionExtractor: funcExtractor,
	}
	if remote.Method == http.MethodGet {
		cfg.PayloadExtractor = fromParams
	} else {
		cfg.PayloadExtractor = fromBody
	}

	if region, ok := ecfg["region"].(string); ok {
		cfg.Region = region
	}
	if endpoint, ok := ecfg["endpoint"].(string); ok {
		cfg.Endpoint = endpoint
	}
	// a JSON number is a float64: the v1 code asked for an int, so max_retries never took effect
	if raw, ok := ecfg["max_retries"]; ok {
		if n, ok := config.Float64(raw); ok && n >= 0 && n == float64(int(n)) {
			retries := int(n)
			cfg.MaxRetries = &retries
		}
	}

	return cfg, nil
}

type Options struct {
	PayloadExtractor  payloadExtractor
	FunctionExtractor functionExtractor
	Region            string // "" takes it from the AWS environment
	Endpoint          string // "" is the AWS endpoint of the region
	MaxRetries        *int   // nil keeps the SDK default
}

type functionExtractor func(*proxy.Request) string

type payloadExtractor func(*proxy.Request) ([]byte, error)

func fromParams(r *proxy.Request) ([]byte, error) {
	buf := new(bytes.Buffer)
	params := map[string]string{}
	for k, v := range r.Params {
		params[strings.ToLower(k)] = v
	}
	err := json.NewEncoder(buf).Encode(params)

	return buf.Bytes(), err
}

func fromBody(r *proxy.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}
