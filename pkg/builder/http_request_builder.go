package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/exgamer/gosdk-core/pkg/constants"
	"github.com/exgamer/gosdk-core/pkg/debug"
	"github.com/exgamer/gosdk-core/pkg/helpers"
	"github.com/gookit/goutil/netutil/httpheader"
	"github.com/motemen/go-loghttp"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrResponseTooLarge Тело ответа больше предела SetMaxResponseBytes
var ErrResponseTooLarge = errors.New("response body exceeds the limit")

//
// ----------------------------------------------------
// Constructors
// ----------------------------------------------------
//

func NewPostHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodPost)
}

func NewPutHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodPut)
}

func NewPatchHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodPatch)
}

func NewGetHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodGet)
}

func NewDeleteHttpRequestBuilder[E any](ctx context.Context, url string) *HttpRequestBuilder[E] {
	return newBuilder[E](ctx, url, http.MethodDelete)
}

func newBuilder[E any](ctx context.Context, rawURL, method string) *HttpRequestBuilder[E] {
	base := http.DefaultTransport.(*http.Transport).Clone()

	return &HttpRequestBuilder[E]{
		url:     rawURL,
		method:  method,
		timeout: 30 * time.Second,
		transport: loghttp.Transport{
			Transport:   base,
			LogRequest:  func(*http.Request) {},  // 🔕 отключили
			LogResponse: func(*http.Response) {}, // 🔕 отключили
		},
		responseDataPresentation: constants.JSON,
		ctx:                      ctx,
		throwUnmarshalError:      true,
	}
}

//
// ----------------------------------------------------
// Builder
// ----------------------------------------------------
//

type HttpRequestBuilder[E any] struct {
	url                      string
	method                   string
	headers                  map[string]string
	queryParams              map[string]string
	body                     io.Reader
	rawBodyBytes             []byte
	timeout                  time.Duration
	transport                loghttp.Transport
	response                 *HttpResponse[E]
	responseDataPresentation string
	ctx                      context.Context
	execTime                 time.Duration
	onComplete               func(ctx context.Context, call CallResult)
	noRedirects              bool
	maxResponseBytes         int64

	throwUnmarshalError bool
}

// CallResult Итог одного вызова для хука SetOnComplete.
// StatusCode 0 — ответа нет (сеть, таймаут, отмена, ошибка сборки запроса); Err — ошибка вызова или nil.
// URL — итоговый адрес с query-параметрами: маскировать секреты в нём — забота хука
type CallResult struct {
	Method       string
	URL          string
	StatusCode   int
	Duration     time.Duration
	Err          error
	RequestBody  []byte
	ResponseBody []byte
}

//
// ----------------------------------------------------
// Configuration
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) SetRequestHeaders(h map[string]string) *HttpRequestBuilder[E] {
	b.headers = h
	return b
}

func (b *HttpRequestBuilder[E]) SetQueryParams(q map[string]string) *HttpRequestBuilder[E] {
	b.queryParams = q
	return b
}

func (b *HttpRequestBuilder[E]) SetRequestTimeout(t time.Duration) *HttpRequestBuilder[E] {
	b.timeout = t
	return b
}

// SetOnComplete Хук после каждого вызова Do (и GetResult): журнал внешних вызовов, метрики. Вызывается один раз,
// в том числе при транспортной ошибке; ошибка разбора тела в GetResult в хук не попадает
func (b *HttpRequestBuilder[E]) SetOnComplete(fn func(ctx context.Context, call CallResult)) *HttpRequestBuilder[E] {
	b.onComplete = fn

	return b
}

// SetNoRedirects Не следовать редиректам: ответ 3xx возвращается как есть (статус, заголовок Location)
func (b *HttpRequestBuilder[E]) SetNoRedirects() *HttpRequestBuilder[E] {
	b.noRedirects = true

	return b
}

// SetMaxResponseBytes Предел тела ответа в байтах: тело больше предела — ошибка ErrResponseTooLarge,
// в HttpResponse.Body — первые n байт. 0 (по умолчанию) — без предела
func (b *HttpRequestBuilder[E]) SetMaxResponseBytes(n int64) *HttpRequestBuilder[E] {
	b.maxResponseBytes = n

	return b
}

func (b *HttpRequestBuilder[E]) SetThrowUnmarshalError(v bool) *HttpRequestBuilder[E] {
	b.throwUnmarshalError = v

	return b
}

//
// ----------------------------------------------------
// Body helpers
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) SetJSONBody(v any) *HttpRequestBuilder[E] {
	data, err := json.Marshal(v)
	if err != nil {
		log.Println(err)

		return b
	}

	b.rawBodyBytes = data
	b.body = bytes.NewReader(data)
	b.ensureHeaders()
	b.headers[httpheader.ContentType] = "application/json"

	return b
}

func (b *HttpRequestBuilder[E]) SetXMLBody(v any) *HttpRequestBuilder[E] {
	data, err := xml.Marshal(v)
	if err != nil {
		log.Println(err)

		return b
	}

	b.rawBodyBytes = data
	b.body = bytes.NewReader(data)
	b.ensureHeaders()
	b.headers[httpheader.ContentType] = "application/xml"

	return b
}

// SetFormBody Тело application/x-www-form-urlencoded (OAuth token endpoint и подобные)
func (b *HttpRequestBuilder[E]) SetFormBody(v url.Values) *HttpRequestBuilder[E] {
	data := []byte(v.Encode())

	b.rawBodyBytes = data
	b.body = bytes.NewReader(data)
	b.ensureHeaders()
	b.headers[httpheader.ContentType] = "application/x-www-form-urlencoded"

	return b
}

//
// ----------------------------------------------------
// Execution
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) Do() error {
	start := time.Now()
	err := b.do()
	b.execTime = time.Since(start)

	if b.response != nil {
		b.response.Duration = b.execTime
	}

	b.notifyComplete(err)

	if err != nil {

		return err
	}

	return nil
}

// Duration Длительность последнего вызова Do
func (b *HttpRequestBuilder[E]) Duration() time.Duration {
	return b.execTime
}

func (b *HttpRequestBuilder[E]) notifyComplete(err error) {
	if b.onComplete == nil {
		return
	}

	call := CallResult{Method: b.method, URL: b.url, Duration: b.execTime, Err: err, RequestBody: b.rawBodyBytes}

	if b.response != nil {
		call.URL = b.response.Url
		call.StatusCode = b.response.StatusCode
		call.ResponseBody = b.response.Body
	}

	b.onComplete(b.ctx, call)
}

func (b *HttpRequestBuilder[E]) do() error {
	b.ensureHeaders()

	client := &http.Client{
		Timeout:   b.timeout,
		Transport: &b.transport,
	}

	if b.noRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}

	finalURL, err := b.buildURL()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(b.ctx, b.method, finalURL, b.body)
	if err != nil {
		return err
	}

	for k, v := range b.headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	b.response = &HttpResponse[E]{Url: finalURL, Method: b.method}
	if err != nil {
		return err
	}

	// тело читается целиком ниже; ошибка закрытия на результат вызова не влияет
	defer func() { _ = resp.Body.Close() }()

	b.response.Status = resp.Status
	b.response.StatusCode = resp.StatusCode
	b.response.Headers = firstHeaderValues(resp.Header)

	return b.readBody(resp.Body)
}

// readBody Тело ответа в b.response.Body; ошибка чтения (обрыв соединения, таймаут) — ошибка вызова
func (b *HttpRequestBuilder[E]) readBody(r io.Reader) error {
	if b.maxResponseBytes <= 0 {
		body, err := io.ReadAll(r)
		b.response.Body = body

		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}

		return nil
	}

	limit := b.maxResponseBytes
	if limit < math.MaxInt64 {
		limit++ // one byte over the limit tells «too large» from «exactly the limit»
	}

	body, err := io.ReadAll(io.LimitReader(r, limit))
	if int64(len(body)) > b.maxResponseBytes {
		b.response.Body = body[:b.maxResponseBytes]

		return fmt.Errorf("%w: more than %d bytes", ErrResponseTooLarge, b.maxResponseBytes)
	}

	b.response.Body = body

	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	return nil
}

//
// ----------------------------------------------------
// Result
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) GetResult() (*HttpResponse[E], error) {
	if err := b.Do(); err != nil {
		return nil, err
	}

	var r E

	if err := json.Unmarshal(b.response.Body, &r); err != nil && b.throwUnmarshalError {
		return nil, err
	}

	b.response.Result = r
	b.setDebugInfo()

	return b.response, nil
}

func (r *HttpResponse[E]) IsServerError() bool {
	return r.StatusCode >= 500
}

func (r *HttpResponse[E]) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

func (r *HttpResponse[E]) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

//
// ----------------------------------------------------
// Logging & Debug
// ----------------------------------------------------
//

func (b *HttpRequestBuilder[E]) setDebugInfo() {
	d := debug.GetDebugFromContext(b.ctx)
	if d == nil {
		return
	}

	st := HttpStatement{
		Time:        helpers.GetDurationAsString(b.execTime),
		Status:      b.response.StatusCode,
		Method:      b.method,
		Url:         maskURL(b.response.Url),
		Headers:     maskHeaders(b.headers),
		QueryParams: maskMap(b.queryParams, sensitiveQuery),
		Body:        string(b.rawBodyBytes),
		Duration:    b.execTime,
	}

	d.Cat("http")
	d.AddStatement("http", b.execTime, []HttpStatement{st})
	d.CalculateTotalTime()
}

//
// ----------------------------------------------------
// Utils
// ----------------------------------------------------
//

// firstHeaderValues Заголовки ответа по первому значению на имя
func firstHeaderValues(h http.Header) map[string]string {
	out := make(map[string]string, len(h))

	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}

	return out
}

// sensitiveHeader Заголовок с секретом: в отладочную информацию — значение «***».
// Ключ идемпотентности (Idempotency-Key, X-Idempotency-Key) — не секрет
func sensitiveHeader(name string) bool {
	n := strings.ToLower(name)

	switch n {
	case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "x-auth-token":
		return true
	case "idempotency-key", "x-idempotency-key":
		return false
	}

	return secretName(n)
}

// secretName Имя (в нижнем регистре) похоже на секрет: оканчивается на -token/_token/-secret/_secret/-key/_key
// или содержит password/apikey
func secretName(n string) bool {
	if strings.Contains(n, "password") || strings.Contains(n, "apikey") {
		return true
	}

	for _, suffix := range []string{"-token", "_token", "-secret", "_secret", "-key", "_key"} {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}

	return false
}

// sensitiveQuery Query-параметр с секретом: в отладочную информацию — значение «***»
func sensitiveQuery(name string) bool {
	n := strings.ToLower(name)

	switch n {
	case "password", "token", "access_token", "refresh_token", "id_token", "client_secret", "code", "state":
		return true
	}

	return strings.HasSuffix(n, "_token") || strings.HasSuffix(n, "_secret") || strings.HasSuffix(n, "_key")
}

// maskHeaders Копия заголовков с замаскированными секретами; исходные заголовки запроса не меняются
func maskHeaders(h map[string]string) map[string]string {
	return maskMap(h, sensitiveHeader)
}

// maskURL Адрес с замаскированными секретными query-параметрами; адрес без них или неразборный — как есть
func maskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return raw
	}

	q := u.Query()
	changed := false

	for k := range q {
		if sensitiveQuery(k) {
			q[k] = []string{"***"}
			changed = true
		}
	}

	if !changed {
		return raw
	}

	u.RawQuery = q.Encode()

	return u.String()
}

// maskMap Копия map, где значения ключей с секретом (secret(k)) заменены на «***»
func maskMap(m map[string]string, secret func(string) bool) map[string]string {
	if m == nil {
		return nil
	}

	out := make(map[string]string, len(m))

	for k, v := range m {
		if secret(k) {
			v = "***"
		}

		out[k] = v
	}

	return out
}

func (b *HttpRequestBuilder[E]) ensureHeaders() {
	if b.headers == nil {
		b.headers = map[string]string{}
	}
}

func (b *HttpRequestBuilder[E]) buildURL() (string, error) {
	if len(b.queryParams) == 0 {
		return b.url, nil
	}

	u, err := url.Parse(b.url)
	if err != nil {
		return "", err
	}

	q := u.Query()
	for k, v := range b.queryParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
