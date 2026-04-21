package common

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

func FetchCertFingerprint(ctx context.Context, address string, timeout time.Duration) (string, error) {
	host, err := normalizeFingerprintDialTarget(address)
	if err != nil {
		return "", err
	}

	dialer := &net.Dialer{Timeout: timeout}
	// Fingerprint capture is TOFU: we must read the presented certificate before trust is established.
	conn, err := tls.DialWithDialer(dialer, "tcp", host, &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	})
	if err != nil {
		return "", fmt.Errorf("tls dial %s: %w", host, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return "", fmt.Errorf("set deadline: %w", err)
	}

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return "", fmt.Errorf("no peer certificate")
	}

	cert := state.PeerCertificates[0]
	sum := sha256.Sum256(cert.Raw)
	fingerprint := strings.ToUpper(hex.EncodeToString(sum[:]))

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	return fingerprint, nil
}

func normalizeFingerprintDialTarget(address string) (string, error) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return "", fmt.Errorf("address must not be empty")
	}

	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("parse address: %w", err)
		}
		hostPart := strings.TrimSpace(parsed.Host)
		if hostPart == "" {
			return "", fmt.Errorf("address host is empty")
		}
		return normalizeHostPort(hostPart)
	}

	withoutPath := strings.SplitN(trimmed, "/", 2)[0]
	return normalizeHostPort(withoutPath)
}

func normalizeHostPort(hostPart string) (string, error) {
	hostPart = strings.TrimSpace(hostPart)
	if hostPart == "" {
		return "", fmt.Errorf("address host is empty")
	}

	host, port, err := net.SplitHostPort(hostPart)
	if err == nil {
		return net.JoinHostPort(strings.Trim(host, "[]"), port), nil
	}

	if strings.HasPrefix(hostPart, "[") && strings.HasSuffix(hostPart, "]") {
		return net.JoinHostPort(strings.Trim(hostPart, "[]"), "443"), nil
	}

	if strings.Count(hostPart, ":") >= 2 {
		return net.JoinHostPort(strings.Trim(hostPart, "[]"), "443"), nil
	}

	if strings.Count(hostPart, ":") == 0 {
		return net.JoinHostPort(hostPart, "443"), nil
	}

	return "", fmt.Errorf("invalid host/port %q: %w", hostPart, err)
}
