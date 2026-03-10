package middleware

import "net/http"

// CookieFixer is a chi middleware that rewrites any Set-Cookie2 response header
// to a standard Set-Cookie header. This works around Huma's struct-based response
// model which only allows one value per header name — by using Set-Cookie2 as a
// secondary slot in the output struct and renaming it here, both cookies are sent
// as proper Set-Cookie headers that browsers understand.
//
// Must be placed before Huma in the chi middleware chain.
func CookieFixer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(&cookieFixerWriter{ResponseWriter: w}, r)
		})
	}
}

type cookieFixerWriter struct {
	http.ResponseWriter
	headerFixed bool
}

func (w *cookieFixerWriter) WriteHeader(code int) {
	w.fixHeaders()
	w.ResponseWriter.WriteHeader(code)
}

func (w *cookieFixerWriter) Write(b []byte) (int, error) {
	w.fixHeaders()
	return w.ResponseWriter.Write(b)
}

func (w *cookieFixerWriter) fixHeaders() {
	if w.headerFixed {
		return
	}
	w.headerFixed = true
	if vals := w.Header().Values("Set-Cookie2"); len(vals) > 0 {
		for _, v := range vals {
			w.Header().Add("Set-Cookie", v)
		}
		w.Header().Del("Set-Cookie2")
	}
}

// Unwrap returns the underlying ResponseWriter for chi middleware compatibility.
func (w *cookieFixerWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
