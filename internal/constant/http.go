// Package constant holds the values shared by several packages: HTTP header names and media types.
package constant

// HTTP header names.
const (
	HeaderContentType     = "Content-Type"
	HeaderContentLength   = "Content-Length"
	HeaderContentEncoding = "Content-Encoding"
	HeaderCacheControl    = "Cache-Control"
	HeaderAuthorization   = "Authorization"
	HeaderUserAgent       = "User-Agent"
	HeaderXForwardedFor   = "X-Forwarded-For"
	// HeaderXFromCache is added to responses that are returned from the cache.
	HeaderXFromCache = "X-From-Cache"
)

// Media types.
const (
	MIMEJSON     = "application/json"
	MIMEJSONUTF8 = "application/json; charset=utf-8"
	MIMEXML      = "application/xml"
	MIMEPlain    = "text/plain"
)
