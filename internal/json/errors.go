package json

import (
	"errors"
	"fmt"

	"github.com/bytedance/sonic/decoder"
)

// DecodeFailure is where an Unmarshal failed and what is wrong, in one line.
type DecodeFailure struct {
	Offset  int
	Message string
}

// DecodeError describes a failed Unmarshal. ok is false when err is not a syntax or type error of the decoder
// (for example an error of a custom UnmarshalJSON method).
func DecodeError(err error) (failure DecodeFailure, ok bool) {
	var syntaxErr decoder.SyntaxError
	if errors.As(err, &syntaxErr) {
		msg := "invalid JSON: " + syntaxErr.Message()
		if syntaxErr.Pos >= 0 && syntaxErr.Pos < len(syntaxErr.Src) && syntaxErr.Message() != "eof" {
			msg += fmt.Sprintf(" %q", syntaxErr.Src[syntaxErr.Pos])
		}

		return DecodeFailure{Offset: syntaxErr.Pos, Message: msg}, true
	}

	var typeErr decoder.MismatchTypeError
	if errors.As(err, &typeErr) {
		return DecodeFailure{Offset: typeErr.Pos, Message: mismatchMessage(typeErr.Src, typeErr.Pos, typeErr.Type.String())}, true
	}

	var typePtrErr *decoder.MismatchTypeError
	if errors.As(err, &typePtrErr) {
		return DecodeFailure{Offset: typePtrErr.Pos, Message: mismatchMessage(typePtrErr.Src, typePtrErr.Pos, typePtrErr.Type.String())}, true
	}

	return DecodeFailure{}, false
}

// mismatchMessage names the kind of JSON value found at pos and the Go type that cannot hold it.
func mismatchMessage(src string, pos int, goType string) string {
	kind := "value"
	if pos >= 0 && pos < len(src) {
		switch c := src[pos]; {
		case c == '[':
			kind = "array"
		case c == '{':
			kind = "object"
		case c == '"':
			kind = "string"
		case c == 't' || c == 'f':
			kind = "bool"
		case c == '-' || (c >= '0' && c <= '9'):
			kind = "number"
		}
	}

	return fmt.Sprintf("cannot unmarshal %s into Go value of type %s", kind, goType)
}
