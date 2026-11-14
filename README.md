# gosdk-http-builder

Universal HTTP Request builder for Go with generics.

Designed for clean, readable, and debuggable HTTP calls in microservices and SDKs.

![Go](https://img.shields.io/badge/go-1.20%2B-blue)
![License](https://img.shields.io/badge/license-MIT-green)

---

## Features

- ✅ HTTP methods: **GET / POST / PUT / PATCH / DELETE**
- ✅ Typed responses via **Go generics**
- ✅ JSON & XML request bodies
- ✅ Headers & query parameters
- ✅ Configurable timeouts
- ✅ Automatic response parsing
- ✅ Optional unmarshal error handling
- ✅ Built‑in debug & timing support
- ✅ Clean builder-style API

---

## Installation

```bash
go get github.com/exgamer/gosdk-http-request-builder/pkg/builder
```

---

## Quick Start

### GET request

```go
ctx := context.Background()

resp, err := builder.
    NewGetHttpbuilder[MyDTO](ctx, "https://api.example.com/v1/items").
    SetQueryParams(map[string]string{
        "page": "1",
        "size": "20",
    }).
    SetRequestHeaders(map[string]string{
        "X-Request-Id": "abc-123",
    }).
    GetResult()

if err != nil {
    panic(err)
}

if resp.IsServerError() {
    //... обработка 500
}

if resp.IsClientError() {
//... обработка 400
}

fmt.Println(resp.StatusCode)
fmt.Println(string(resp.Body))
fmt.Printf("%+v\n", resp.Result)
```

---

## POST JSON

```go
type CreateReq struct {
    Name string `json:"name"`
}

resp, err := builder.
    NewPostHttpbuilder[MyDTO](ctx, "https://api.example.com/v1/items").
    SetJSONBody(CreateReq{Name: "test"}).
    SetRequestHeaders(map[string]string{
        "Authorization": "Bearer token",
    }).
    GetResult()

if err != nil {
    panic(err)
}

if resp.IsServerError() {
//... обработка 500
}

if resp.IsClientError() {
//... обработка 400
}
```

---

## Constructors

```go
NewGetHttpbuilder[E any](ctx context.Context, url string)
NewPostHttpbuilder[E any](ctx context.Context, url string)
NewPutHttpbuilder[E any](ctx context.Context, url string)
NewPatchHttpbuilder[E any](ctx context.Context, url string)
```

---

## Configuration

### Headers

```go
builder.SetRequestHeaders(map[string]string{
    "Authorization": "Bearer ...",
})
```

### Query Parameters

```go
builder.SetQueryParams(map[string]string{
    "limit": "50",
    "sort": "desc",
})
```

### Timeout

```go
builder.SetRequestTimeout(5 * time.Second)
```

Default: **30s**

---

### Ignore JSON Unmarshal Errors

```go
builder.SetThrowUnmarshalError(false)
```

Useful when API may return non‑JSON responses (HTML, plain text, etc).

---

### After-call hook

```go
builder.SetOnComplete(func(ctx context.Context, call builder.CallResult) {
    // call.Method, call.URL (with query — mask secrets yourself), call.StatusCode (0 — no response),
    // call.Duration, call.Err, call.RequestBody, call.ResponseBody
})
```

Called once after every `Do()` / `GetResult()`, including transport errors. A JSON unmarshal error of `GetResult()` is
not passed to the hook.

### Redirects

```go
builder.SetNoRedirects() // a 3xx response is returned as is (status, Location); by default redirects are followed
```

### Response size limit

```go
builder.SetMaxResponseBytes(1 << 20) // body over 1 MiB — error builder.ErrResponseTooLarge, Body holds the first 1 MiB
```

By default there is no limit. A read error of the response body (connection cut, timeout) is returned by `Do()` /
`GetResult()` since v1.3.0.

### Debug info

With a debug collector in the context, secrets of the request are recorded as `***`. Headers: `Authorization`,
`Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `X-Auth-Token`, names ending in `-token`, `_token`,
`-secret`, `_secret`, `-key`, `_key` and names containing `password` or `apikey` (an idempotency key —
`Idempotency-Key`, `X-Idempotency-Key` — is kept). Query (`QueryParams` and the URL): `password`, `token`,
`access_token`, `refresh_token`, `id_token`, `client_secret`, `code`, `state` and names ending in `_token`, `_secret`,
`_key`. The request itself is sent with the real values.

---

## Request Body

### JSON

```go
builder.SetJSONBody(payload)
```

Automatically sets:

```
Content-Type: application/json
```

---

### XML

```go
builder.SetXMLBody(payload)
```

Automatically sets:

```
Content-Type: application/xml
```

---

### Form (application/x-www-form-urlencoded)

```go
builder.SetFormBody(url.Values{"grant_type": {"client_credentials"}, "scope": {"read"}})
```

Automatically sets:

```
Content-Type: application/x-www-form-urlencoded
```

---

## Execution

### Do()

Executes request **without parsing** response body.

```go
err := builder.Do()
```

---

### GetResult()

Executes request and parses response into `Response[E]`.

```go
resp, err := builder.GetResult()
```

`resp.Headers` — response headers, the first value per name (canonical key: `resp.Headers["Retry-After"]`);
`resp.Duration` (and `builder.Duration()`) — call duration from the start of `Do()` to the end of reading the body.

Behavior:
- Executes request
- Parses JSON response
- Adds debug statement (if enabled)
- Returns error on **HTTP 5xx**
- Does **not** treat 4xx as error

---

## Examples

### PUT

```go
builder.
    NewPutHttpbuilder[MyDTO](ctx, url).
    SetJSONBody(data).
    GetResult()
```

---

### PATCH

```go
builder.
    NewPatchHttpbuilder[MyDTO](ctx, url).
    SetJSONBody(data).
    GetResult()
```

---

### POST XML

```go
builder.
    NewPostHttpbuilder[MyDTO](ctx, url).
    SetXMLBody(xmlData).
    GetResult()
```

---

## Error Handling

```go
resp, err := builder.GetResult()
if err != nil {
    return err
}

if resp.StatusCode >= 400 {
    return fmt.Errorf(
        "bad response: %d %s",
        resp.StatusCode,
        string(resp.Body),
    )
}
```

### Error sources

- Invalid URL
- Request creation error
- Network error
- JSON unmarshal error (optional)
- HTTP **5xx** responses

---

## Debug & Tracing

If `debug` object is present in `context.Context`, builder automatically records:

- HTTP method & URL
- Headers & query params
- Request body
- Status code
- Execution time

Useful for request aggregation & performance analysis.

---

## Design Notes

- 4xx responses are **business logic**, not transport errors
- 5xx responses are treated as **errors**
- Logging transport is disabled by default
- Content-Type is set automatically

---

## Roadmap / Ideas

- Bearer & Basic auth helpers
- Retry / backoff
- ThrowOn4xx option
- Raw body support
- XML response parsing
- Middleware hooks

---

## License

MIT License © Exgamer
