package auth

// ClientInfo is a client as shown by the admin API. It never contains key
// material beyond the display hint. Timestamps are RFC 3339 (UTC).
type ClientInfo struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	KeyHint   string  `json:"key_hint"`
	Status    string  `json:"status"` // "aktiv" or "gesperrt"
	CreatedAt string  `json:"created_at"`
	RotatedAt *string `json:"rotated_at" tstype:"string | null,required"`
}

// CreateClientRequest is the body of POST /api/clients.
type CreateClientRequest struct {
	Name string `json:"name"`
}

// CreateClientResponse is returned by create and rotate. Key is the plaintext
// client key and is shown exactly once.
type CreateClientResponse struct {
	Client ClientInfo `json:"client"`
	Key    string     `json:"key"`
}

// ListClientsResponse is the body of GET /api/clients.
type ListClientsResponse struct {
	Clients []ClientInfo `json:"clients"`
}

// ClientResponse is returned by block.
type ClientResponse struct {
	Client ClientInfo `json:"client"`
}

// ErrorResponse is the error body of the admin API.
type ErrorResponse struct {
	Error string `json:"error"`
}
