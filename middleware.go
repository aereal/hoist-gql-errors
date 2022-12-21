package hoistgqlgenerrors

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// New returns the middleware function that extracts GraphQL errors from
// downstream http.Handler.
//
// The extracted errors are recorded span's errors.
func New() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			span := trace.SpanFromContext(r.Context())
			if !span.IsRecording() {
				next.ServeHTTP(w, r)
				return
			}
			rw := newResponseRecorder(w)
			next.ServeHTTP(rw, r)
			var resp graphql.Response
			if err := json.NewDecoder(rw.body).Decode(&resp); err != nil {
				return
			}
			if len(resp.Errors) == 0 {
				return
			}
			span.SetStatus(codes.Error, resp.Errors.Error())
			for _, err := range resp.Errors {
				span.RecordError(err)
			}
		})
	}
}

func newResponseRecorder(base http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		base: base,
		body: new(bytes.Buffer),
	}
}

type responseRecorder struct {
	base http.ResponseWriter
	body *bytes.Buffer
}

var _ http.ResponseWriter = &responseRecorder{}

func (r *responseRecorder) Header() http.Header {
	return r.base.Header()
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.base.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	_, _ = r.body.Write(b)
	return r.base.Write(b)
}
