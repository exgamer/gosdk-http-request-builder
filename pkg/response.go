package httprequestbuilder

type Response[E interface{}] struct {
	Success bool `json:"success"`
	Data    E    `json:"data"`
}
