// Modifier plugin of the benchmark: the request modifier adds a header for the backend, the response modifier adds
// a field to the decoded body. It uses only the standard library, so the same source is built for inkgate and for
// KrakenD.
package main

import "fmt"

// ModifierRegisterer is the symbol both gateways look for.
var ModifierRegisterer = registerer("bench-modifier")

type registerer string

func (registerer) RegisterLogger(any) {}

func (r registerer) RegisterModifiers(f func(
	name string,
	modifierFactory func(map[string]any) func(any) (any, error),
	appliesToRequest bool,
	appliesToResponse bool,
)) {
	f(string(r)+"-request", requestModifier, true, false)
	f(string(r)+"-response", responseModifier, false, true)
}

type request interface {
	Headers() map[string][]string
}

type response interface {
	Data() map[string]any
}

func requestModifier(map[string]any) func(any) (any, error) {
	return func(input any) (any, error) {
		req, ok := input.(request)
		if !ok {
			return nil, fmt.Errorf("unknown request type %T", input)
		}
		req.Headers()["X-Bench-Req"] = []string{"1"}

		return input, nil
	}
}

func responseModifier(map[string]any) func(any) (any, error) {
	return func(input any) (any, error) {
		resp, ok := input.(response)
		if !ok {
			return nil, fmt.Errorf("unknown response type %T", input)
		}
		if data := resp.Data(); data != nil {
			data["bench"] = "modifier"
		}

		return input, nil
	}
}

func main() {}
