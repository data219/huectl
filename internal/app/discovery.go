package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func DiscoverBridges(ctx context.Context, timeout time.Duration) ([]map[string]any, error) {
	results := map[string]map[string]any{}

	ssdpResults, err := discoverSSDP(ctx, timeout)
	if err == nil {
		for _, row := range ssdpResults {
			key := row["address"].(string)
			results[key] = row
		}
	}

	mdnsResults, err := discoverMDNS(timeout)
	if err == nil {
		for _, row := range mdnsResults {
			key := row["address"].(string)
			if _, exists := results[key]; !exists {
				results[key] = row
			}
		}
	}

	flattened := make([]map[string]any, 0, len(results))
	for _, row := range results {
		flattened = append(flattened, row)
	}
	return flattened, nil
}

func discoverSSDP(ctx context.Context, timeout time.Duration) ([]map[string]any, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	request := strings.Join([]string{
		"M-SEARCH * HTTP/1.1",
		"HOST: 239.255.255.250:1900",
		"MAN: \"ssdp:discover\"",
		"MX: 2",
		"ST: upnp:rootdevice",
		"",
		"",
	}, "\r\n")
	_, err = conn.WriteTo([]byte(request), &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900})
	if err != nil {
		return nil, fmt.Errorf("send ssdp query: %w", err)
	}

	results := map[string]map[string]any{}
	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return mapValues(results), nil
		default:
		}

		n, _, readErr := conn.ReadFrom(buf)
		if readErr != nil {
			if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
				return mapValues(results), nil
			}
			return nil, fmt.Errorf("read ssdp response: %w", readErr)
		}

		headers := parseSSDPHeaders(string(buf[:n]))
		location := headers["LOCATION"]
		if location == "" || (!strings.Contains(strings.ToLower(location), "hue") && !strings.Contains(strings.ToLower(location), "philips")) {
			continue
		}
		addr := locationToHost(location)
		if addr == "" {
			continue
		}
		results[addr] = map[string]any{
			"address":  addr,
			"location": location,
			"source":   "ssdp",
			"usn":      headers["USN"],
		}
	}
}

func discoverMDNS(timeout time.Duration) ([]map[string]any, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	query := buildMDNSQuery("_hue._tcp.local")
	_, err = conn.WriteTo(query, &net.UDPAddr{IP: net.ParseIP("224.0.0.251"), Port: 5353})
	if err != nil {
		return nil, err
	}

	results := map[string]map[string]any{}
	buf := make([]byte, 4096)
	for {
		n, _, readErr := conn.ReadFrom(buf)
		if readErr != nil {
			if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
				return mapValues(results), nil
			}
			return nil, readErr
		}
		payload := buf[:n]
		if strings.Contains(strings.ToLower(string(payload)), "hue") {
			// mDNS parsing is intentionally lightweight; bridge address is resolved via A-record hints in packet text.
			if addr := parseMDNSAddress(payload); addr != "" {
				results[addr] = map[string]any{"address": addr, "source": "mdns"}
			}
		}
	}
}

func parseSSDPHeaders(raw string) map[string]string {
	lines := strings.Split(raw, "\r\n")
	headers := map[string]string{}
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(parts[0]))
		headers[key] = strings.TrimSpace(parts[1])
	}
	return headers
}

func locationToHost(location string) string {
	url := strings.TrimSpace(location)
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	if idx := strings.Index(url, "/"); idx >= 0 {
		url = url[:idx]
	}
	if url == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(url)
	if err == nil {
		return host
	}
	return url
}

func mapValues(input map[string]map[string]any) []map[string]any {
	rows := make([]map[string]any, 0, len(input))
	for _, value := range input {
		rows = append(rows, value)
	}
	return rows
}

func buildMDNSQuery(name string) []byte {
	// Minimal query packet for PTR lookup, sufficient for local discovery best-effort.
	packet := []byte{
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" {
			continue
		}
		packet = append(packet, byte(len(label)))
		packet = append(packet, []byte(label)...)
	}
	packet = append(packet, 0x00)
	packet = append(packet, 0x00, 0x0c)
	packet = append(packet, 0x00, 0x01)
	return packet
}

func parseMDNSAddress(payload []byte) string {
	// Best-effort extraction: prefer any IPv4 literal present in payload text rendering.
	text := string(payload)
	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r == '.' || (r >= '0' && r <= '9'))
	}) {
		if net.ParseIP(token) != nil {
			return token
		}
	}
	return ""
}

func discoverMeethue() ([]map[string]any, error) {
	// Optional fallback if local discovery does not return results.
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://discovery.meethue.com/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return nil, nil
}
