package api_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	vgrpc "github.com/AlexGalhardo/vukem/contrib/grpc"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/api"
)

func TestGRPCPortProductionRule(t *testing.T) {
	t.Parallel()

	cert, key := selfSignedFiles(t)
	h := http.NotFoundHandler()

	tests := []struct {
		name    string
		env     config.Env
		h2c     bool
		port    api.GRPCPort
		wantRun bool
		wantErr error
	}{
		{name: "development serves h2c", env: config.Development, port: api.GRPCPort{}, wantRun: true},
		{name: "off", env: config.Development, port: api.GRPCPort{Port: "off"}},
		{
			name: "production without tls is refused", env: config.Production, port: api.GRPCPort{},
			wantErr: vgrpc.ErrCleartextInProduction,
		},
		{
			name: "HTTP_H2C alone does not open the grpc port", env: config.Production, h2c: true,
			port: api.GRPCPort{}, wantErr: vgrpc.ErrCleartextInProduction,
		},
		{
			name: "production with the cleartext opt-in", env: config.Production,
			port: api.GRPCPort{InsecureCleartext: true}, wantRun: true,
		},
		{
			name: "production with tls files", env: config.Production,
			port: api.GRPCPort{CertFile: cert, KeyFile: key}, wantRun: true,
		},
		{
			name: "unreadable tls files fail", env: config.Production,
			port:    api.GRPCPort{CertFile: cert, KeyFile: filepath.Join(t.TempDir(), "missing.pem")},
			wantErr: os.ErrNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := vukem.Config{Env: tt.env, H2C: tt.h2c}
			run, _, err := tt.port.Runner(&cfg, h)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if got := run != nil; got != tt.wantRun {
				t.Errorf("runner = %v, want %v", got, tt.wantRun)
			}
		})
	}
}

func TestGRPCPortServesAndDrains(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.Handle(grpchealth.NewHandler(grpchealth.NewStaticChecker()))

	ready := make(chan string, 1)
	port := api.GRPCPort{Host: "127.0.0.1", Port: "0", Ready: ready}

	run, _, err := port.Runner(&vukem.Config{Env: config.Development, ShutdownTimeout: 5 * time.Second}, mux)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { done <- run.Run(ctx) }()

	addr := <-ready

	// Prior-knowledge h2c, as native gRPC clients speak to a cleartext port.
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)

	client := &http.Client{Transport: &http.Transport{Protocols: &protocols}, Timeout: 5 * time.Second}
	health := grpchealth.NewClient(client, "http://"+addr, connect.WithGRPC())

	res, err := health.Check(t.Context(), &grpchealth.CheckRequest{})
	if err != nil || res.Status != grpchealth.StatusServing {
		t.Fatalf("health over h2c = %v, %v", res, err)
	}

	cancel()

	if err := <-done; err != nil {
		t.Errorf("Run after cancel = %v, want nil (graceful drain)", err)
	}

	if _, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		t.Error("port still accepts connections after shutdown")
	}
}

// selfSignedFiles writes a throwaway certificate and key for localhost.
func selfSignedFiles(t *testing.T) (string, string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")

	if err := os.WriteFile(
		certFile,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		keyFile,
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	return certFile, keyFile
}
