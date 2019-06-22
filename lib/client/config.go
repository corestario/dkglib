package client

const (
	flagGet = "get"

	// DefaultKeyPass contains the default key password for genesis transactions
	DefaultKeyPass = "12345678"
)

var configDefaults = map[string]string{
	"chain-id":       "",
	"output":         "text",
	"node":           "tcp://localhost:26657",
	"broadcast-mode": "sync",
}
