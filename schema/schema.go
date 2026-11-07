// Package schema holds the JSON schema of schema.json, embedded in the binary for `check --lint-no-network`.
package schema

import _ "embed"

// JSON is the schema document. It is a placeholder ({}) until a schema is written.
//
//go:embed schema.json
var JSON string
