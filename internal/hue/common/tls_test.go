package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchCertFingerprintSupportsSelfSignedCertificates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cert := server.Certificate()
	sum := sha256.Sum256(cert.Raw)
	expected := strings.ToUpper(hex.EncodeToString(sum[:]))

	fingerprint, err := FetchCertFingerprint(context.Background(), server.URL, 3*time.Second)
	if err != nil {
		t.Fatalf("fetch fingerprint: %v", err)
	}
	if fingerprint != expected {
		t.Fatalf("unexpected fingerprint: got=%q want=%q", fingerprint, expected)
	}
}

func TestFetchCertFingerprintReturnsErrorForUnreachableHost(t *testing.T) {
	_, err := FetchCertFingerprint(context.Background(), "127.0.0.1:1", 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
	if !strings.Contains(err.Error(), "tls dial") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchCertFingerprintHonorsCanceledContext(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchCertFingerprint(ctx, server.URL, 3*time.Second)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if err != context.Canceled {
		t.Fatalf("unexpected error: got=%v want=%v", err, context.Canceled)
	}
}

func TestNormalizeFingerprintDialTarget(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{name: "ipv4 without port", address: "192.168.1.2", want: "192.168.1.2:443"},
		{name: "ipv4 with port", address: "192.168.1.2:8443", want: "192.168.1.2:8443"},
		{name: "https ipv4", address: "https://192.168.1.2", want: "192.168.1.2:443"},
		{name: "https ipv4 with path", address: "https://192.168.1.2/clip/v2", want: "192.168.1.2:443"},
		{name: "ipv6 unbracketed", address: "2001:db8::1", want: "[2001:db8::1]:443"},
		{name: "ipv6 bracketed", address: "[2001:db8::1]", want: "[2001:db8::1]:443"},
		{name: "ipv6 bracketed with port", address: "[2001:db8::1]:8443", want: "[2001:db8::1]:8443"},
		{name: "https ipv6 bracketed with port", address: "https://[2001:db8::1]:8443", want: "[2001:db8::1]:8443"},
		{name: "https ipv6 unbracketed", address: "https://2001:db8::1", want: "[2001:db8::1]:443"},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			got, err := normalizeFingerprintDialTarget(testCase.address)
			if err != nil {
				t.Fatalf("normalizeFingerprintDialTarget error: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("unexpected dial target: got=%q want=%q", got, testCase.want)
			}
		})
	}
}
