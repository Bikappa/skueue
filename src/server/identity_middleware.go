package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type contextKey string

const (
	identityContextKey contextKey = "identity"
)

const identityHeader string = "arduino-user-id"

// IdentityMiddleware ensure a user is passed in the request, add it to the context
func IdentityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func ContextIdentity(ctx context.Context) string {
	v := ctx.Value(identityContextKey)
	if v == nil {
		panic(fmt.Errorf("invalid access to identity in context"))
	}
	return v.(string)
}
