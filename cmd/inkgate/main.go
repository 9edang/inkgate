// Command inkgate is the inkgate API gateway built on the gorilla/mux router.
package main

import (
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/flow/boot"
	"github.com/9edang/inkgate/flow/entry"
)

func main() {
	// gorilla/mux matches /users/{id} natively, so endpoint paths keep their brackets.
	config.RoutingPattern = config.BracketsRouterPatternBuilder
	boot.Main(entry.Router)
}
