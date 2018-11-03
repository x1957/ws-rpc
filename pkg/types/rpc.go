package types

import "encoding/json"

type Request struct {
	Method string          `json:"method"`
	Args   json.RawMessage `json:"args,omitempty"` // TODO consider other proto
}

type Response struct {
	Status int    `json:"status"` // 0 ok, 1 ping, > 1 errors
	Data   []byte `json:"data,omitempty"`
}
