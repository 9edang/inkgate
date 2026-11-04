// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"net/http"
)

func main() {}

var ModifierRegisterer = registerer("inkgate-error-example")

var logger Logger = nil

type registerer string

func (r registerer) RegisterModifiers(f func(
	name string,
	modifierFactory func(map[string]any) func(any) (any, error),
	appliesToRequest bool,
	appliesToResponse bool,
)) {
	f(string(r)+"-request", r.requestModifierFactory, true, false)
	f(string(r)+"-response", r.reqsponseModifierFactory, false, true)
}

func (registerer) RegisterLogger(in any) {
	l, ok := in.(Logger)
	if !ok {
		return
	}
	logger = l
	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Logger loaded", ModifierRegisterer))
}

func (registerer) requestModifierFactory(_ map[string]any) func(any) (any, error) {
	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Request modifier injected", ModifierRegisterer))
	return func(_ any) (any, error) {
		logger.Debug(fmt.Sprintf("[PLUGIN: %s] Rejecting request", ModifierRegisterer))
		return nil, requestErr
	}
}

func (registerer) reqsponseModifierFactory(_ map[string]any) func(any) (any, error) {
	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Response modifier injected", ModifierRegisterer))
	return func(_ any) (any, error) {
		logger.Debug(fmt.Sprintf("[PLUGIN: %s] Replacing response", ModifierRegisterer))
		return nil, responseErr
	}
}

type customError struct {
	error
	statusCode int
}

func (r customError) StatusCode() int { return r.statusCode }

var (
	requestErr = customError{
		error:      errors.New("request rejected just because"),
		statusCode: http.StatusTeapot,
	}
	responseErr = customError{
		error:      errors.New("response replaced because reasons"),
		statusCode: http.StatusTeapot,
	}
)

type Logger interface {
	Debug(v ...any)
	Info(v ...any)
	Warning(v ...any)
	Error(v ...any)
	Critical(v ...any)
	Fatal(v ...any)
}
