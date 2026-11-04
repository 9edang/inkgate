package json

import (
	stdjson "encoding/json"
)

// RawMessage and Number are the encoding/json types; sonic encodes and decodes
// them natively, so values stay interchangeable with code using the std ones.
type (
	RawMessage = stdjson.RawMessage
	Number     = stdjson.Number
)
