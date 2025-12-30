# gosdk-http-requestbuilder

Universal HTTP Request Builder for Go with generics.

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
go get github.com/exgamer/gosdk-http-request-buildere/pkg/requestbuilder
```

---

## Quick Start

### GET request

```go
ctx := context.Background()

resp, err := requestbuilder.
    NewGetHttpRequestBuilder[MyDTO](ctx, "https://api.example.com/v1/items").
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

resp, err := requestbuilder.
    NewPostHttpRequestBuilder[MyDTO](ctx, "https://api.example.com/v1/items").
    SetJSONBody(CreateReq{Name: "test"}).
    SetRequestHeaders(map[string]string{
        "Authorization": "Bearer token",
    }).
    GetResult()
```

---

## Constructors

```go
NewGetHttpRequestBuilder[E any](ctx context.Context, url string)
NewPostHttpRequestBuilder[E any](ctx context.Context, url string)
NewPutHttpRequestBuilder[E any](ctx context.Context, url string)
NewPatchHttpRequestBuilder[E any](ctx context.Context, url string)
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
requestbuilder.
    NewPutHttpRequestBuilder[MyDTO](ctx, url).
    SetJSONBody(data).
    GetResult()
```

---

### PATCH

```go
requestbuilder.
    NewPatchHttpRequestBuilder[MyDTO](ctx, url).
    SetJSONBody(data).
    GetResult()
```

---

### POST XML

```go
requestbuilder.
    NewPostHttpRequestBuilder[MyDTO](ctx, url).
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
