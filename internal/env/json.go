package env

func JSON() ConfigJSON {
	return json
}

type ConfigJSON struct {
	// UseCustomConfig also overwrites the global sonic.ConfigStd with the API
	// built from the options below, so gin (which binds JSON through
	// sonic.ConfigStd) inherits them too. It does not affect what this package
	// itself uses — the options below apply either way.
	UseCustomConfig bool `env:"JSON_USE_CUSTOM_CONFIG" envDefault:"false"`

	// EscapeHTML escapes HTML characters (<, >, &) when encoding, matching
	// encoding/json. Costs significant performance.
	EscapeHTML bool `env:"JSON_ESCAPE_HTML" envDefault:"false"`
	// SortMapKeys sorts map keys before encoding, as encoding/json always
	// does. On by default because signatures (JWS), cache keys and query
	// strings built from marshaled maps need byte-stable output. Costs some
	// performance; turn off only where map order does not matter.
	SortMapKeys bool `env:"JSON_SORT_MAP_KEYS" envDefault:"true"`
	// CompactMarshaler skips validating that json.Marshaler output is compact.
	CompactMarshaler bool `env:"JSON_COMPACT_MARSHALER" envDefault:"false"`
	// NoQuoteTextMarshaler skips quoting encoding.TextMarshaler output.
	NoQuoteTextMarshaler bool `env:"JSON_NO_QUOTE_TEXT_MARSHALER" envDefault:"false"`
	// NoNullSliceOrMap encodes nil slices/maps as [] and {} instead of null.
	// Only two encoded fields in this module lack omitempty
	// (ciam authenticateSubmitRequestBody.Callbacks,
	// esb TelkomSubmitOrderRequest.Orders) and both are always populated, so
	// this changes almost nothing here — off by default to keep the outgoing
	// wire format exactly as upstream has always received it.
	NoNullSliceOrMap bool `env:"JSON_NO_NULL_SLICE_OR_MAP" envDefault:"false"`

	// UseInt64 decodes JSON integers into `any` as int64 rather than float64.
	// The one option defaulted on, because with float64 integers beyond 2^53
	// lose precision silently — verified, 10000000000000001 decodes back as
	// 10000000000000000. This module decodes into `any` in two places —
	// httpclient's response-body logging and ciam's CallbackField.Value
	// (which is echoed back upstream) — so the default protects both. Typed
	// struct fields are unaffected either way.
	UseInt64 bool `env:"JSON_USE_INT64" envDefault:"true"`
	// UseNumber decodes numbers into `any` as json.Number, preserving the
	// exact literal. Takes precedence over UseInt64 for non-integers too, at
	// the cost of callers having to convert.
	UseNumber bool `env:"JSON_USE_NUMBER" envDefault:"false"`
	// UseUnicodeErrors makes the decoder error on invalid UTF-8 escapes
	// instead of substituting replacement characters.
	UseUnicodeErrors bool `env:"JSON_USE_UNICODE_ERRORS" envDefault:"false"`
	// DisallowUnknownFields makes the decoder error when the input contains
	// object keys with no matching destination field. Off by default: the
	// upstream IFAs add response fields without notice, and erroring on them
	// would break working calls.
	DisallowUnknownFields bool `env:"JSON_DISALLOW_UNKNOWN_FIELDS" envDefault:"false"`
	// CopyString makes the decoder copy string values instead of referencing
	// the input buffer. Costs an allocation per string; only needed if a
	// decoded string outlives the buffer it was decoded from.
	CopyString bool `env:"JSON_COPY_STRING" envDefault:"false"`
	// ValidateString makes the decoder/encoder reject unescaped control
	// characters (U+0000 through U+001F) inside strings. Off by default:
	// enabling it turns responses that currently decode fine into hard
	// errors.
	ValidateString bool `env:"JSON_VALIDATE_STRING" envDefault:"false"`
	// NoValidateJSONMarshaler skips validating json.Marshaler output.
	NoValidateJSONMarshaler bool `env:"JSON_NO_VALIDATE_JSON_MARSHALER" envDefault:"false"`
	// NoValidateJSONSkip skips validation of JSON values being skipped over
	// (unknown fields, mismatched types, redundant elements).
	NoValidateJSONSkip bool `env:"JSON_NO_VALIDATE_JSON_SKIP" envDefault:"false"`
	// NoEncoderNewline omits the trailing newline Encoder.Encode writes after
	// each value.
	NoEncoderNewline bool `env:"JSON_NO_ENCODER_NEWLINE" envDefault:"false"`
	// EncodeNullForInfOrNan encodes +Inf, -Inf and NaN floats as null instead
	// of failing the whole marshal.
	EncodeNullForInfOrNan bool `env:"JSON_ENCODE_NULL_FOR_INF_OR_NAN" envDefault:"false"`
	// CaseSensitive makes the decoder match object keys case-sensitively.
	// Off by default: several IFA responses vary key casing between examples
	// (e.g. ciam CheckAccountResponse's transactionID/transactionid), and the
	// case-insensitive fallback is what makes a single struct tag cover both.
	CaseSensitive bool `env:"JSON_CASE_SENSITIVE" envDefault:"false"`
}
