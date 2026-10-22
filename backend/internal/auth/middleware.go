package auth

import (
	"net/http"
	"time"

	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

func RequireAuth(next http.Handler) http.Handler {
	return clerkhttp.RequireHeaderAuthorization(
		clerkhttp.Leeway(5 * time.Minute),
	)(next)
}
