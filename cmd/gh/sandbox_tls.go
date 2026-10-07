package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// init installs a custom CA bundle for every HTTPS call when GH_SSL_CERT_FILE
// (or SSL_CERT_FILE as a fallback) points at a PEM file. If neither is set it is
// a no-op.
//
// Why: in some sandboxed environments (e.g. certain agent sandboxes on macOS)
// the `trustd` mach service that backs Security.framework is unreachable. Go's
// TLS stack defers to that system verifier when RootCAs is nil, so every HTTPS
// request from gh fails with a certificate error. Pointing at an explicit PEM CA
// bundle makes Go verify with that pool using its own pure-Go path instead of
// trustd, restoring TLS inside the sandbox.
//
// gh's HTTP clients (REST, GraphQL, and anything built via go-gh's
// api.NewHTTPClient) start from http.DefaultTransport, so replacing it here
// applies to all of them.
func init() {
	path := os.Getenv("GH_SSL_CERT_FILE")
	if path == "" {
		path = os.Getenv("SSL_CERT_FILE")
	}
	if path == "" {
		return
	}

	pem, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gh: ignoring CA bundle %q (GH_SSL_CERT_FILE/SSL_CERT_FILE): %v\n", path, err)
		return
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		fmt.Fprintf(os.Stderr, "gh: ignoring CA bundle %q: no certificates found\n", path)
		return
	}

	// Clone http.DefaultTransport so we keep its proxy/timeout/HTTP2 settings and
	// only override the root CAs.
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return
	}
	transport := base.Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.RootCAs = pool
	http.DefaultTransport = transport
}
