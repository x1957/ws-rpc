package types

type Request struct {
	Method string `json:"method"`
	Args   []byte `json:"args,omitempty"`
}

type Response struct {
	Status int    `json:"status"` // 0 ok, 1 ping, > 1 errors
	Data   []byte `json:"data,omitempty"`
}
