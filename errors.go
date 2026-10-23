package witan

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIError is a non-2xx answer from the API. Use errors.As to reach it.
type APIError struct {
	Status  int             // the HTTP status
	Message string          // the server's "error" field, or the status text
	Body    json.RawMessage // the answer as sent
}

func (e *APIError) Error() string { return fmt.Sprintf("witan: %d %s", e.Status, e.Message) }

// PaymentRequiredError is a 402: a priced unit or a paid dataset version not yet bought, or a quota
// exceeded. Pay is the x402 URL to buy it at, Price the amount; buying is a wallet's x402 payment or,
// with a key, the operator's credits.
type PaymentRequiredError struct {
	APIError
	Price       string
	Pay         string
	FreeWithKey bool // the seller set no price: an agent key reads it free
}

func (e *PaymentRequiredError) Error() string {
	if e.Pay != "" {
		return fmt.Sprintf("witan: 402 %s (pay %s at %s)", e.Message, e.Price, e.Pay)
	}
	return e.APIError.Error()
}

// Unwrap lets errors.As find the APIError too.
func (e *PaymentRequiredError) Unwrap() error { return &e.APIError }

func newAPIError(status int, raw []byte) error {
	var b struct {
		Error       any    `json:"error"`
		Price       string `json:"price"`
		Pay         string `json:"pay"`
		FreeWithKey bool   `json:"freeWithKey"`
	}
	_ = json.Unmarshal(raw, &b)
	msg := http.StatusText(status)
	switch v := b.Error.(type) {
	case string:
		if v != "" {
			msg = v
		}
	case map[string]any:
		if m, ok := v["message"].(string); ok && m != "" {
			msg = m
		}
	}
	base := APIError{Status: status, Message: msg, Body: json.RawMessage(raw)}
	if status == http.StatusPaymentRequired {
		return &PaymentRequiredError{APIError: base, Price: b.Price, Pay: b.Pay, FreeWithKey: b.FreeWithKey}
	}
	return &base
}
