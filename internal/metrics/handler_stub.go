//go:build no_api

package metrics

// When no_api build tag is set, the HTTP metrics handler is excluded
// to eliminate dependencies on net/http and crypto/tls.
