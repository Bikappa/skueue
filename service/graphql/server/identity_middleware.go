package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler/transport"
)

type contextKey string

const (
	identityContextKey contextKey = "identity"
)

const identityHeader string = "arduino-user-id"

// IdentityHttpMiddleware ensure a user is passed in the request, add it to the context
func IdentityHttpMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Connection") == "Upgrade" {
			// ws must be handled differently
			next.ServeHTTP(w, r)
			return
		}

		identityheaderValue := r.Header.Get(identityHeader)
		if identityheaderValue == "" {
			w.WriteHeader(http.StatusBadRequest)
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			enc.Encode(map[string]string{
				"error": "missing identity header",
			})
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, identityContextKey, identityheaderValue)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func IdentityWsInitFunc(ctx context.Context, initPayload transport.InitPayload) (context.Context, error) {

	identityheaderValue := initPayload.GetString(identityHeader)
	if identityheaderValue == "" {
		return ctx, fmt.Errorf("missing identity")
	}
	ctx = context.WithValue(ctx, identityContextKey, identityheaderValue)
	return ctx, nil
}

func ContextIdentity(ctx context.Context) string {
	v := ctx.Value(identityContextKey)
	if v == nil {
		panic(fmt.Errorf("invalid access to identity in context"))
	}
	return v.(string)
}
