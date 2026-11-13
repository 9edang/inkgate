// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"sort"
	"strings"
)

// Get returns the value of an extra_config namespace by its code name or by any alias of it ("router/stream" and
// "inkgate/engine/router/stream" are the same namespace). It works before and after Normalize.
func (e ExtraConfig) Get(name string) (any, bool) {
	if v, ok := e[name]; ok {
		return v, true
	}
	for alias, long := range ExtraConfigAlias {
		if long == name {
			if v, ok := e[alias]; ok {
				return v, true
			}
		}
	}

	return nil, false
}

// UnknownNamespaceError is an extra_config key that no feature reads: a typo, or a feature inkgate does not have.
// Left alone it turns the feature off without any message.
type UnknownNamespaceError struct {
	Where      string // "service", `endpoint GET /users`, `endpoint GET /users, backend 1`...
	Namespace  string
	Suggestion string // the closest known namespace, "" when none is close
}

func (e *UnknownNamespaceError) Error() string {
	msg := fmt.Sprintf("%s: unknown extra_config namespace %q", e.Where, e.Namespace)
	if e.Suggestion != "" {
		msg += fmt.Sprintf(" (did you mean %q?)", e.Suggestion)
	}

	return msg
}

// ValidateNamespaces checks the extra_config of the service, its endpoints, async agents and their backends against
// the namespaces the build supports, and returns one error per unknown key, naming where it is. It reads the keys as
// written (aliases or code names), so it works before Normalize. Level is not checked: a known namespace in a place
// that does not read it is not reported.
func (s *ServiceConfig) ValidateNamespaces(known []string) []error {
	set := make(map[string]bool, len(known))
	for _, k := range known {
		set[k] = true
	}
	var errs []error
	check := func(where string, extra ExtraConfig) {
		keys := make([]string, 0, len(extra))
		for k := range extra {
			if !set[k] {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			errs = append(errs, &UnknownNamespaceError{Where: where, Namespace: k, Suggestion: Suggest(k, known)})
		}
	}
	backends := func(where string, list []*Backend) {
		for i, b := range list {
			if b != nil {
				check(fmt.Sprintf("%s, backend %d (%s)", where, i+1, b.URLPattern), b.ExtraConfig)
			}
		}
	}

	check("service", s.ExtraConfig)
	for _, e := range s.Endpoints {
		if e == nil {
			continue
		}
		where := fmt.Sprintf("endpoint %s %s", e.Method, e.Endpoint)
		check(where, e.ExtraConfig)
		backends(where, e.Backend)
	}
	for _, a := range s.AsyncAgents {
		if a == nil {
			continue
		}
		where := fmt.Sprintf("async_agent %q", a.Name)
		check(where, a.ExtraConfig)
		backends(where, a.Backend)
	}

	return errs
}

// Suggest returns the candidate closest to name, or "" when none is close enough to be a likely typo.
func Suggest(name string, candidates []string) string {
	best, bestDist := "", 0
	for _, c := range candidates {
		d := editDistance(name, c)
		if best == "" || d < bestDist {
			best, bestDist = c, d
		}
	}
	// within 3 edits, and no more than a third of the name: "router/stram" is a typo of "router/stream", "foo" is not
	// a typo of anything
	if best == "" || bestDist > 3 || bestDist*3 > len(strings.TrimSpace(name)) {
		return ""
	}

	return best
}

// editDistance is the optimal string alignment distance between a and b: Levenshtein, where swapping two adjacent
// characters ("mdoe" for "mode") counts as one edit, as it is the most common typo.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}

	return d[len(ra)][len(rb)]
}
