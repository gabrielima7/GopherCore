// Package httpkit provides an HTTP toolkit built on go-chi/chi with
// pre-configured security middleware, rate limiting, CORS control,
// and standardized JSON responses.
// Purpose: Manages HTTP request routing and server lifecycle initialization securely.
// Constraints: Requires strict parameter configuration to defend against Slowloris.
// Thread-safety: Router execution and server spinning are safe for multi-core multiplexing.
// Internal Logic Deep-Dive: The router natively enforces non-zero Read/Write timeouts directly on the underlying tcp listener, mitigating Slowloris and connection-drain attacks at the transport layer before headers are even parsed.
package httpkit
