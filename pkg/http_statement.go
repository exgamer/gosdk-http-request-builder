package httprequestbuilder

import "time"

type HttpStatement struct {
	Time        string                 `json:"time,omitempty"`
	Status      int                    `json:"status,omitempty"`
	Timeout     string                 `json:"timeout,omitempty"`
	Method      string                 `json:"method,omitempty"`
	Url         string                 `json:"url,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Body        string                 `json:"body,omitempty"`
	QueryParams map[string]string      `json:"query_params,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Response    map[string]interface{} `json:"response,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
}
