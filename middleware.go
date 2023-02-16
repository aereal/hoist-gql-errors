package hoistgqlgenerrors

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/semconv/v1.17.0/httpconv"
	"go.opentelemetry.io/otel/trace"
)

type Option func(*config)

// WithEventOptions returns an Option that passes given trace.EventOption to span.RecordError().
func WithEventOptions(opts ...trace.EventOption) Option {
	return func(c *config) {
		c.eventOptions = append(c.eventOptions, opts...)
	}
}

type EventOptionsBuilderFunc func(req *http.Request, statusCode int, responseHeader http.Header) []trace.EventOption

// WithEventOptionsBuilder return an Option that uses the function to build attributes for the error events.
func WithEventOptionsBuilder(builderFunc EventOptionsBuilderFunc) Option {
	return func(c *config) {
		c.builderFuncs = append(c.builderFuncs, builderFunc)
	}
}

// WithHTTPConventionalAttributes return an Option that adds the attributes conforms to HTTP semantic conventions to the error events.
func WithHTTPConventionalAttributes() Option {
	return func(c *config) {
		c.builderFuncs = append(c.builderFuncs, withHTTPConventionalAttributes)
	}
}

func withHTTPConventionalAttributes(req *http.Request, statusCode int, responseHeader http.Header) []trace.EventOption {
	attrs := []attribute.KeyValue{semconv.HTTPStatusCode(statusCode)}
	attrs = append(attrs, httpconv.RequestHeader(req.Header)...)
	attrs = append(attrs, httpconv.ResponseHeader(responseHeader)...)
	attrs = append(attrs, httpconv.ServerRequest("", req)...)
	return []trace.EventOption{trace.WithAttributes(attrs...)}
}

type config struct {
	eventOptions []trace.EventOption
	builderFuncs []EventOptionsBuilderFunc
}

// New returns the middleware function that extracts GraphQL errors from
// downstream http.Handler.
//
// The extracted errors are recorded span's errors.
func New(opts ...Option) func(http.Handler) http.Handler {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
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
				eventOptions := cfg.eventOptions
				for _, builder := range cfg.builderFuncs {
					eventOptions = append(eventOptions, builder(r, rw.statusCode, rw.Header())...)
				}
				span.RecordError(err, eventOptions...)
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
	base          http.ResponseWriter
	body          *bytes.Buffer
	statusCode    int
	headerWritten bool
}

var _ http.ResponseWriter = &responseRecorder{}

func (r *responseRecorder) Header() http.Header {
	return r.base.Header()
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.headerWritten = true
	r.statusCode = statusCode
	r.base.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.headerWritten {
		r.WriteHeader(http.StatusOK)
	}
	_, _ = r.body.Write(b)
	return r.base.Write(b)
}
