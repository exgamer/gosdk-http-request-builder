package builder

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/exgamer/gosdk-core/pkg/debug"
)

func TestSetFormBody_SendsURLEncoded(t *testing.T) {
	var gotType, gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	_, err := NewPostHttpRequestBuilder[echoResult](context.Background(), srv.URL).
		SetFormBody(url.Values{"grant_type": {"client_credentials"}, "scope": {"a b"}}).
		GetResult()
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}

	if gotType != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q", gotType)
	}

	if gotBody != "grant_type=client_credentials&scope=a+b" {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestSetNoRedirects_Returns3xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/next" {
			_, _ = w.Write([]byte(`{"id":"followed"}`))

			return
		}

		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer srv.Close()

	b := NewGetHttpRequestBuilder[echoResult](context.Background(), srv.URL).SetNoRedirects()
	if err := b.Do(); err != nil {
		t.Fatalf("Do: %v", err)
	}

	if b.response.StatusCode != http.StatusFound || b.response.Headers["Location"] != "/next" {
		t.Fatalf("status %d, Location %q — want 302 /next", b.response.StatusCode, b.response.Headers["Location"])
	}

	// by default redirects are still followed (compatibility)
	resp, err := NewGetHttpRequestBuilder[echoResult](context.Background(), srv.URL).GetResult()
	if err != nil || resp.Result.ID != "followed" {
		t.Fatalf("default: %v %+v — want the redirect followed", err, resp)
	}
}

func TestSetMaxResponseBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()

	b := NewGetHttpRequestBuilder[echoResult](context.Background(), srv.URL).SetMaxResponseBytes(10)
	err := b.Do()

	if !errors.Is(err, ErrResponseTooLarge) || len(b.response.Body) != 10 {
		t.Fatalf("err %v, body %d bytes — want ErrResponseTooLarge and 10 bytes", err, len(b.response.Body))
	}

	exact := NewGetHttpRequestBuilder[echoResult](context.Background(), srv.URL).SetMaxResponseBytes(100)
	if err := exact.Do(); err != nil || len(exact.response.Body) != 100 {
		t.Fatalf("exact limit: %v, %d bytes", err, len(exact.response.Body))
	}

	unlimited := NewGetHttpRequestBuilder[echoResult](context.Background(), srv.URL)
	if err := unlimited.Do(); err != nil || len(unlimited.response.Body) != 100 {
		t.Fatalf("no limit: %v, %d bytes", err, len(unlimited.response.Body))
	}
}

func TestDo_ReadErrorReturned(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}

		buf := make([]byte, 1024)
		_, _ = c.Read(buf)
		// promises 100 bytes, sends 5 and closes — a cut response
		_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\nhello"))
		_ = c.Close()
	}()

	b := NewGetHttpRequestBuilder[echoResult](context.Background(), "http://"+ln.Addr().String())
	if err := b.Do(); err == nil {
		t.Fatal("Do: want a read error for a cut body, got nil")
	}
}

func TestDebugInfo_MasksSecretHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer real" {
			w.WriteHeader(http.StatusUnauthorized)
		}

		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	dbg := debug.NewDebugCollector()
	ctx := debug.WithDebugCollector(context.Background(), dbg)

	resp, err := NewGetHttpRequestBuilder[echoResult](ctx, srv.URL).
		SetRequestHeaders(map[string]string{"Authorization": "Bearer real", "X-Api-Key": "k1", "X-Request-Id": "r1"}).
		GetResult()
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("the real request must carry the real header: %v %+v", err, resp)
	}

	sts := dbg.Cat("http").Statements
	if len(sts) != 1 {
		t.Fatalf("statements = %d", len(sts))
	}

	h := sts[0].([]HttpStatement)[0].Headers
	if h["Authorization"] != "***" || h["X-Api-Key"] != "***" || h["X-Request-Id"] != "r1" {
		t.Fatalf("debug headers = %v — want secrets masked, others kept", h)
	}
}

func TestSetMaxResponseBytes_MaxInt64IsNoLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	b := NewGetHttpRequestBuilder[echoResult](context.Background(), srv.URL).SetMaxResponseBytes(math.MaxInt64)
	if err := b.Do(); err != nil || string(b.response.Body) != "hello" {
		t.Fatalf("MaxInt64 limit: %v, body %q — want the whole body", err, b.response.Body)
	}
}

func TestSensitiveNames(t *testing.T) {
	headers := map[string]bool{
		"Authorization": true, "Proxy-Authorization": true, "Cookie": true, "Set-Cookie": true, "X-Api-Key": true,
		"X-Auth-Token": true, "X-Refresh-Token": true, "X_Client_Secret": true, "X-Signing-Key": true,
		"X-Password": true, "Apikey": true, "Idempotency-Key": false, "X-Idempotency-Key": false,
		"X-Request-Id": false, "Content-Type": false, "X-Keyboard": false,
	}
	for name, want := range headers {
		if got := sensitiveHeader(name); got != want {
			t.Errorf("sensitiveHeader(%q) = %v, want %v", name, got, want)
		}
	}

	query := map[string]bool{
		"password": true, "token": true, "access_token": true, "refresh_token": true, "id_token": true,
		"client_secret": true, "code": true, "state": true, "api_key": true, "page": false, "q": false,
		"keyword": false,
	}
	for name, want := range query {
		if got := sensitiveQuery(name); got != want {
			t.Errorf("sensitiveQuery(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestDebugInfo_MasksSecretQuery(t *testing.T) {
	var gotCode string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCode = r.URL.Query().Get("code")
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	dbg := debug.NewDebugCollector()
	ctx := debug.WithDebugCollector(context.Background(), dbg)

	_, err := NewGetHttpRequestBuilder[echoResult](ctx, srv.URL).
		SetQueryParams(map[string]string{"code": "c1", "page": "2"}).
		SetRequestHeaders(map[string]string{"X-Idempotency-Key": "i1"}).
		GetResult()
	if err != nil || gotCode != "c1" {
		t.Fatalf("the real request must carry the real query: %v, code %q", err, gotCode)
	}

	st := dbg.Cat("http").Statements[0].([]HttpStatement)[0]
	if st.QueryParams["code"] != "***" || st.QueryParams["page"] != "2" {
		t.Fatalf("debug query = %v", st.QueryParams)
	}

	if strings.Contains(st.Url, "c1") || !strings.Contains(st.Url, "page=2") {
		t.Fatalf("debug url = %q — want the code masked, page kept", st.Url)
	}

	if st.Headers["X-Idempotency-Key"] != "i1" {
		t.Fatalf("idempotency key masked: %v", st.Headers)
	}
}
