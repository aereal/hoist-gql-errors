package hoistgqlgenerrors_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	hoistgqlgenerrors "github.com/aereal/hoist-gql-errors"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

func TestMiddleware(t *testing.T) {
	t.Run("some error", func(t *testing.T) {
		event := sdktrace.Event{
			Name: semconv.ExceptionEventName,
			Attributes: []attribute.KeyValue{
				semconv.ExceptionTypeKey.String("*gqlerror.Error"),
				semconv.ExceptionMessageKey.String("input: oops"),
			},
		}
		doTest(t, withTrace, gqlerror.List{gqlerror.Errorf("oops")}, []tracetest.SpanStub{{Name: "/", Events: []sdktrace.Event{event}}})
	})
	t.Run("no error returned", func(t *testing.T) {
		doTest(t, withTrace, gqlerror.List{}, []tracetest.SpanStub{{Name: "/"}})
	})
	t.Run("no span started", func(t *testing.T) {
		doTest(t, passthrough, gqlerror.List{gqlerror.Errorf("oops")}, []tracetest.SpanStub{})
	})
}

func doTest(t *testing.T, traceMiddleware func(tp *sdktrace.TracerProvider) middleware, errs gqlerror.List, wantSpans []tracetest.SpanStub) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	mw := hoistgqlgenerrors.New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := graphql.Response{Errors: errs}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(traceMiddleware(tp)(mw(handler)))
	defer srv.Close()
	ctx := context.Background()
	if err := doRequest(ctx, srv.URL); err != nil {
		t.Fatalf("doRequest: %s", err)
	}
	if err := tp.ForceFlush(ctx); err != nil {
		t.Errorf("ForceFlush: %s", err)
	}
	spans := exporter.GetSpans()
	if diff := cmpSpans(wantSpans, spans); diff != "" {
		t.Errorf("spans (-want, +got):\n%s", diff)
	}
}

func transformSpanStub(span tracetest.SpanStub) map[string]any {
	return map[string]any{
		"Name":       span.Name,
		"Attributes": span.Attributes,
		"Events":     span.Events,
	}
}

func transformAttributeValue(value attribute.Value) any {
	return value.AsInterface()
}

func cmpSpans(want, got tracetest.SpanStubs) string {
	opts := []cmp.Option{
		cmp.Transformer("SpanStub", transformSpanStub),
		cmp.Transformer("attribute.Value", transformAttributeValue),
		cmpopts.IgnoreFields(sdktrace.Event{}, "Time"),
	}
	return cmp.Diff(want, got, opts...)
}

type middleware = func(http.Handler) http.Handler

func withTrace(tp *sdktrace.TracerProvider) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tp.Tracer("test").Start(r.Context(), r.URL.Path)
			defer span.End()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func passthrough(_ *sdktrace.TracerProvider) middleware {
	return func(next http.Handler) http.Handler {
		return next
	}
}

func doRequest(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}