package boot

import (
	rss "github.com/9edang/inkgate/feature/transform/rss"
	xml "github.com/9edang/inkgate/feature/transform/xml"
)

// RegisterEncoders enables the backend decoders beyond JSON (XML, RSS).
// Output renderers are router-specific and registered by the variant.
func RegisterEncoders() {
	// #nosec G104 -- error is not actionable here
	_ = xml.Register()
	// #nosec G104 -- error is not actionable here
	_ = rss.Register()
}
