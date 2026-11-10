package builder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type echoResult struct {
	ID string `json:"id"`
}

func TestGetResult_FillsHeadersAndDuration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.Header().Add("X-Multi", "first")
		w.Header().Add("X-Multi", "second")
		time.Sleep(5 * time.Millisecond)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	b := NewGetHttpRequestBuilder[echoResult](context.Background(), srv.URL)

	resp, err := b.GetResult()
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}

	if resp.Headers["Retry-After"] != "120" {
		t.Fatalf("Headers[Retry-After] = %q, want 120; headers %v", resp.Headers["Retry-After"], resp.Headers)
	}

	if resp.Headers["X-Multi"] != "first" {
		t.Fatalf("Headers[X-Multi] = %q, want the first value", resp.Headers["X-Multi"])
	}

	if resp.Duration < 5*time.Millisecond || resp.Duration != b.Duration() {
		t.Fatalf("Duration = %v, builder Duration = %v, want ≥ 5ms and equal", resp.Duration, b.Duration())
	}

	if resp.Result.ID != "1" {
		t.Fatalf("Result = %+v", resp.Result)
	}
}

func TestOnComplete_CalledOnceWithResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"id":"2"}`))
	}))
	defer srv.Close()

	type ctxKey struct{}

	ctx := context.WithValue(context.Background(), ctxKey{}, "req")

	var calls []CallResult

	var gotCtx context.Context

	_, err := NewPostHttpRequestBuilder[echoResult](ctx, srv.URL).
		SetQueryParams(map[string]string{"q": "1"}).
		SetJSONBody(map[string]string{"a": "b"}).
		SetOnComplete(func(c context.Context, call CallResult) {
			gotCtx = c
			calls = append(calls, call)
		}).
		GetResult()
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("hook calls = %d, want 1", len(calls))
	}

	call := calls[0]

	if gotCtx.Value(ctxKey{}) != "req" {
		t.Fatal("hook got another context")
	}

	if call.Method != http.MethodPost || call.URL != srv.URL+"?q=1" || call.StatusCode != http.StatusConflict || call.Err != nil {
		t.Fatalf("call = %+v", call)
	}

	if string(call.RequestBody) != `{"a":"b"}` || string(call.ResponseBody) != `{"id":"2"}` || call.Duration <= 0 {
		t.Fatalf("bodies or duration: %+v", call)
	}
}

func TestOnComplete_TransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	var calls []CallResult

	_, err := NewGetHttpRequestBuilder[echoResult](context.Background(), url).
		SetOnComplete(func(_ context.Context, call CallResult) { calls = append(calls, call) }).
		GetResult()
	if err == nil {
		t.Fatal("GetResult: want a transport error")
	}

	if len(calls) != 1 || calls[0].Err == nil || calls[0].StatusCode != 0 || calls[0].URL != url {
		t.Fatalf("calls = %+v, want one call with Err and StatusCode 0", calls)
	}
}
