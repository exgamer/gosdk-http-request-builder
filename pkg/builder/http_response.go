package builder

import "time"

// HttpResponse Модель описывающая ответ от rest запроса.
// Headers — заголовки ответа, по первому значению на имя (ключ — каноническое имя, http.CanonicalHeaderKey);
// Duration — длительность вызова от начала Do до прочтения тела ответа
type HttpResponse[E interface{}] struct {
	Status     string
	Body       []byte
	StatusCode int
	Url        string
	Method     string
	Headers    map[string]string
	Result     E
	ErrorsMap  map[string]interface{}
	Duration   time.Duration
}
