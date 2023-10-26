package json

type AuthRS struct {
	Errors
	Token      string `json:"token"`
	TokenType  string `json:"token_type"`
	Expiration int    `json:"expiration"`
}
