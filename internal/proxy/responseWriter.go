package proxy

import "net/http"

type responseWriter struct {
	http.ResponseWriter
	code int
}
