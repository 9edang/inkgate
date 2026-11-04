package mux

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/internal/json"
	"log"
	"net/http"
	"net/textproto"
	"regexp"
	"strings"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	muxrouter "github.com/9edang/inkgate/engine/router/mux"
	inkgatejose "github.com/9edang/inkgate/feature/auth/jose"
	"github.com/9edang/inkgate/internal/auth0"
	"github.com/go-jose/go-jose/v3/jwt"
)

func HandlerFactory(hf muxrouter.HandlerFactory, paramExtractor muxrouter.ParamExtractor, logger logging.Logger, rejecterF inkgatejose.RejecterFactory) muxrouter.HandlerFactory {
	return TokenSignatureValidator(TokenSigner(hf, paramExtractor, logger), logger, rejecterF)
}

func TokenSigner(hf muxrouter.HandlerFactory, paramExtractor muxrouter.ParamExtractor, logger logging.Logger) muxrouter.HandlerFactory {
	return func(cfg *config.EndpointConfig, prxy proxy.Proxy) http.HandlerFunc {
		signerCfg, signer, err := inkgatejose.NewSigner(cfg, nil)
		if err == inkgatejose.ErrNoSignerCfg {
			logger.Info("JOSE: signer disabled for the endpoint", cfg.Endpoint)
			return hf(cfg, prxy)
		}
		if err != nil {
			logger.Error("JOSE: unable to create the signer for the endpoint", cfg.Endpoint)
			logger.Error(err.Error())
			return hf(cfg, prxy)
		}

		logger.Info("JOSE: signer enabled for the endpoint", cfg.Endpoint)

		return func(w http.ResponseWriter, r *http.Request) {
			proxyReq := muxrouter.NewRequestBuilder(paramExtractor)(r, cfg.QueryString, cfg.HeadersToPass)
			ctx, cancel := context.WithTimeout(r.Context(), cfg.Timeout)
			defer cancel()

			response, err := prxy(ctx, proxyReq)
			if err != nil {
				logger.Error("proxy response error:", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if response == nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := inkgatejose.SignFields(signerCfg.KeysToSign, signer, response); err != nil {
				logger.Error(err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			for k, v := range response.Metadata.Headers {
				w.Header().Set(k, v[0])
			}

			err = jsonRender(w, response)
			if err != nil {
				logger.Error("render answer error:", err.Error())
			}
		}
	}
}

var emptyResponse = []byte("{}")

func jsonRender(w http.ResponseWriter, response *proxy.Response) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if response == nil {
		_, err := w.Write(emptyResponse)
		return err
	}

	// static or merged responses carry no status; WriteHeader(0) panics
	if response.Metadata.StatusCode != 0 {
		w.WriteHeader(response.Metadata.StatusCode)
	}

	js, err := json.Marshal(response.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	_, err = w.Write(js)
	return err
}

func TokenSignatureValidator(hf muxrouter.HandlerFactory, logger logging.Logger, rejecterF inkgatejose.RejecterFactory) muxrouter.HandlerFactory {
	return func(cfg *config.EndpointConfig, prxy proxy.Proxy) http.HandlerFunc {
		if rejecterF == nil {
			rejecterF = new(inkgatejose.NopRejecterFactory)
		}
		rejecter := rejecterF.New(logger, cfg)

		handler := hf(cfg, prxy)
		requiredClaims := extractRequiredJWTClaims(cfg)
		jwtParams := func(r *http.Request, claims map[string]any) *http.Request {
			if len(requiredClaims) == 0 {
				return r
			}
			cl := inkgatejose.Claims(claims)
			params := make(map[string]string, len(requiredClaims))
			for _, name := range requiredClaims {
				if v, ok := cl.Get(name); ok {
					params["JWT."+name] = v
				}
			}
			return muxrouter.WithParams(r, params)
		}
		signatureConfig, err := inkgatejose.GetSignatureConfig(cfg)
		if err == inkgatejose.ErrNoValidatorCfg {
			logger.Info("JOSE: validator disabled for the endpoint", cfg.Endpoint)
			return handler
		}
		if err != nil {
			logger.Warning(fmt.Sprintf("JOSE: validator for %s: %s", cfg.Endpoint, err.Error()))
			return handler
		}

		validator, err := inkgatejose.NewValidator(signatureConfig, FromCookie, FromHeader)
		if err != nil {
			log.Fatalf("%s: %s", cfg.Endpoint, err.Error())
		}

		var aclCheck func(string, map[string]any, []string) bool

		if signatureConfig.RolesKeyIsNested && strings.Contains(signatureConfig.RolesKey, ".") && signatureConfig.RolesKey[:4] != "http" {
			aclCheck = inkgatejose.CanAccessNested
		} else {
			aclCheck = inkgatejose.CanAccess
		}

		var scopesMatcher func(string, map[string]any, []string) bool

		if len(signatureConfig.Scopes) > 0 && signatureConfig.ScopesKey != "" {
			if signatureConfig.ScopesMatcher == "all" {
				scopesMatcher = inkgatejose.ScopesAllMatcher
			} else {
				scopesMatcher = inkgatejose.ScopesAnyMatcher
			}
		} else {
			scopesMatcher = inkgatejose.ScopesDefaultMatcher
		}

		logger.Info("JOSE: validator enabled for the endpoint", cfg.Endpoint)

		return func(w http.ResponseWriter, r *http.Request) {
			token, err := validator.ValidateRequest(r)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			claims := map[string]any{}
			err = validator.Claims(r, token, &claims)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if rejecter.Reject(claims) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !aclCheck(signatureConfig.RolesKey, claims, signatureConfig.Roles) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			if !scopesMatcher(signatureConfig.ScopesKey, claims, signatureConfig.Scopes) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			propagateHeaders(cfg, signatureConfig.PropagateClaimsToHeader, signatureConfig.PropagateClaimsPreserveArray, claims, r, logger)
			context.From(r.Context()).SetClaims(claims)

			handler(w, jwtParams(r, claims))
		}
	}
}

func FromCookie(key string) func(r *http.Request) (*jwt.JSONWebToken, error) {
	if key == "" {
		key = "access_token"
	}
	return func(r *http.Request) (*jwt.JSONWebToken, error) {
		cookie, err := r.Cookie(key)
		if err != nil {
			return nil, auth0.ErrTokenNotFound
		}
		return jwt.ParseSigned(cookie.Value)
	}
}

func FromHeader(header string) func(r *http.Request) (*jwt.JSONWebToken, error) {
	if header == "" {
		header = "Authorization"
	}
	return func(r *http.Request) (*jwt.JSONWebToken, error) {
		raw := r.Header.Get(header)
		if len(raw) > 7 && strings.EqualFold(raw[0:7], "BEARER ") {
			raw = raw[7:]
		}
		if raw == "" {
			return nil, auth0.ErrTokenNotFound
		}
		return jwt.ParseSigned(raw)
	}
}

func propagateHeaders(
	cfg *config.EndpointConfig,
	propagationCfg [][]string,
	propagationPreserveArrays bool,
	claims map[string]any,
	r *http.Request,
	logger logging.Logger,
) {
	if len(propagationCfg) > 0 {
		if !propagationPreserveArrays {
			headersToPropagate, err := inkgatejose.CalculateHeadersToPropagate(propagationCfg, claims)
			if err != nil {
				logger.Warning(fmt.Sprintf("JOSE: header propagations error for %s: %s", cfg.Endpoint, err.Error()))
			}
			for k, v := range headersToPropagate {
				// Set header value - replaces existing one
				r.Header.Set(k, v)
			}
			return
		}

		headersToPropagate, err := inkgatejose.CalculateArrayHeadersToPropagate(propagationCfg, claims)
		if err != nil {
			logger.Warning(fmt.Sprintf("JOSE: header propagations error for %s: %s", cfg.Endpoint, err.Error()))
		}
		for k, v := range headersToPropagate {
			r.Header[textproto.CanonicalMIMEHeaderKey(k)] = v
		}
	}
}

var jwtParamsPattern = regexp.MustCompile(`{{\.JWT\.([^}]*)}}`)

// extractRequiredJWTClaims returns the claims used as {{.JWT.<claim>}} in the
// endpoint's backend url_patterns.
func extractRequiredJWTClaims(cfg *config.EndpointConfig) []string {
	var required []string
	for _, backend := range cfg.Backend {
		for _, match := range jwtParamsPattern.FindAllStringSubmatch(backend.URLPattern, -1) {
			if len(match) >= 2 {
				required = append(required, match[1])
			}
		}
	}
	return required
}
