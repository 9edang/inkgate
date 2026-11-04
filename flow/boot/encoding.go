package boot

import (
	rss "github.com/9edang/inkgate/feature/transform/rss"
	xml "github.com/9edang/inkgate/feature/transform/xml"
)

// RegisterEncoders enables the backend decoders beyond JSON (XML, RSS).
// Output renderers are router-specific and registered by the variant.
func RegisterEncoders() {
	xml.Register()
	rss.Register()
}
