package types

type Response struct {
	Code string `json:"code"`
	Data any    `json:"data,omitempty"`
}
