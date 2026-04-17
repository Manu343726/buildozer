package daemon

import "fmt"

// RpcURL constructs the HTTP RPC URL from daemon host and port.
// This should be used whenever connecting to a daemon's RPC endpoints.
// Example: RpcURL("localhost", 6789) returns "http://localhost:6789"
func RpcURL(host string, port int) string {
	return fmt.Sprintf("http://%s:%d", host, port)
}
