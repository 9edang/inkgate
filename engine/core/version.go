// SPDX-License-Identifier: Apache-2.0

/*
Package core contains some basic constants and variables
*/
package core

import (
	"fmt"
	"runtime"
	"strings"
)

// InkgateHeaderName is the name of the custom Inkgate header
const InkgateHeaderName = "X-INKGATE"

// InkgateVersion is the version of the build
var InkgateVersion = "undefined"

// GoVersion is the version of the go compiler used at build time
var GoVersion = strings.TrimPrefix(runtime.Version(), "go")

// GlibcVersion is the version of the glibc used by CGO at build time
var GlibcVersion = "undefined"

// InkgateHeaderValue is the value of the custom Inkgate header
var InkgateHeaderValue = fmt.Sprintf("Version %s", InkgateVersion)

// InkgateUserAgent is the value of the user agent header sent to the backends
var InkgateUserAgent = fmt.Sprintf("Inkgate Version %s", InkgateVersion)
