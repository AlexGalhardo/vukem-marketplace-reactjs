package api

import (
	"cmp"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	vgrpc "github.com/AlexGalhardo/vukem/contrib/grpc"
)

// DefaultGRPCPort is used when GRPC_PORT is unset.
const DefaultGRPCPort = "50051"

// GRPCPort configures the server of the RPC services on their own port.
type GRPCPort struct {
	Host string
	// Port is GRPC_PORT: "" means DefaultGRPCPort, "off" serves nothing.
	Port string
	// CertFile and KeyFile (GRPC_TLS_CERT_FILE, GRPC_TLS_KEY_FILE) serve
	// HTTP/2 over TLS; without both the port speaks h2c.
	CertFile, KeyFile string
	// InsecureCleartext (GRPC_INSECURE_CLEARTEXT=true, vukem's name; the
	// older GRPC_INSECURE_PLAINTEXT is still read) allows h2c in production,
	// for a private network or a proxy that terminates TLS.
	InsecureCleartext bool
	// Ready, when set, receives the bound address once listening.
	Ready chan<- string
}

// GRPCPortFrom reads the port settings from src.
func GRPCPortFrom(src *config.Source) GRPCPort {
	return GRPCPort{
		Host:              src.Get("HOST"),
		Port:              src.Get("GRPC_PORT"),
		CertFile:          src.Get("GRPC_TLS_CERT_FILE"),
		KeyFile:           src.Get("GRPC_TLS_KEY_FILE"),
		InsecureCleartext: src.Get("GRPC_INSECURE_CLEARTEXT") == "true" || src.Get("GRPC_INSECURE_PLAINTEXT") == "true",
	}
}

// Runner returns the runner serving h and its address, or a nil runner
// when the port is off. API keys travel in metadata, so production
// without TLS files or InsecureCleartext returns
// vgrpc.ErrCleartextInProduction.
func (p GRPCPort) Runner(cfg *vukem.Config, h http.Handler) (vukem.Runner, string, error) {
	if p.Port == "off" {
		return nil, "", nil
	}

	addr := net.JoinHostPort(p.Host, cmp.Or(p.Port, DefaultGRPCPort))
	opts := vgrpc.ServerOptions{
		ShutdownTimeout:   cfg.ShutdownTimeout,
		Ready:             p.Ready,
		InsecureCleartext: p.InsecureCleartext,
	}

	if p.CertFile != "" && p.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(p.CertFile, p.KeyFile)
		if err != nil {
			return nil, addr, fmt.Errorf("grpc tls: %w", err)
		}

		opts.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}

	run, err := vgrpc.Server(cfg, addr, h, opts)
	if err != nil {
		return nil, addr, fmt.Errorf("grpc port %s: %w", addr, err)
	}

	return run, addr, nil
}
