package json

import (
	"github.com/9edang/inkgate/internal/env"
	"io"
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/option"
)

// json is the frozen sonic API all functions in this package delegate to,
// built once at init from Config (see config.go). Go initializes imported
// packages before their importers, so every caller sees the env-configured
// API — there is no window where an unconfigured default is in use.
var json sonic.API

func init() {
	cfg := env.JSON()
	json = sonic.Config{
		EscapeHTML:              cfg.EscapeHTML,
		SortMapKeys:             cfg.SortMapKeys,
		CompactMarshaler:        cfg.CompactMarshaler,
		NoQuoteTextMarshaler:    cfg.NoQuoteTextMarshaler,
		NoNullSliceOrMap:        cfg.NoNullSliceOrMap,
		UseInt64:                cfg.UseInt64,
		UseNumber:               cfg.UseNumber,
		UseUnicodeErrors:        cfg.UseUnicodeErrors,
		DisallowUnknownFields:   cfg.DisallowUnknownFields,
		CopyString:              cfg.CopyString,
		ValidateString:          cfg.ValidateString,
		NoValidateJSONMarshaler: cfg.NoValidateJSONMarshaler,
		NoValidateJSONSkip:      cfg.NoValidateJSONSkip,
		NoEncoderNewline:        cfg.NoEncoderNewline,
		EncodeNullForInfOrNan:   cfg.EncodeNullForInfOrNan,
		CaseSensitive:           cfg.CaseSensitive,
	}.Froze()

}

// Valid reports whether data is a syntactically valid JSON encoding.
func Valid(data []byte) bool {
	return json.Valid(data)
}

// Marshal returns the JSON encoding of v.
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// MarshalIndent returns the JSON encoding of v with each element prefixed
// by prefix and indented by one or more copies of indent, mirroring
// encoding/json.MarshalIndent.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

// MarshalToString returns the JSON encoding of v as a string.
func MarshalToString(v any) (string, error) {
	return json.MarshalToString(v)
}

// Unmarshal parses the JSON-encoded data and stores the result in v.
func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// UnmarshalFromString parses the JSON-encoded string and stores the result in v.
func UnmarshalFromString(data string, v any) error {
	return json.UnmarshalFromString(data, v)
}

// NewEncoder returns a streaming JSON encoder that writes to w.
func NewEncoder(w io.Writer) sonic.Encoder {
	return json.NewEncoder(w)
}

// NewDecoder returns a streaming JSON decoder that reads from r.
func NewDecoder(r io.Reader) sonic.Decoder {
	return json.NewDecoder(r)
}

// Pretouch pre-compiles the sonic JIT decoder/encoder for the given type.
// Call at startup for every type that will be marshaled or unmarshaled to
// eliminate first-request JIT latency spikes.
func Pretouch[T any](recursiveDepth, maxInlineDepth int) error {
	return sonic.Pretouch(reflect.TypeFor[T](),
		option.WithCompileRecursiveDepth(recursiveDepth),
		option.WithCompileMaxInlineDepth(maxInlineDepth))
}
