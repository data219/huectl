package common

import (
	"strings"

	"github.com/data219/huectl/internal/domain"
)

func BridgeBaseV2(bridge domain.Bridge) string {
	if bridge.APIBaseV2 != "" {
		return strings.TrimSuffix(bridge.APIBaseV2, "/")
	}
	base := normalizeBridgeAddress(bridge.Address)
	if strings.HasSuffix(base, "/clip/v2") {
		return base
	}
	return base + "/clip/v2"
}

func BridgeBaseV1(bridge domain.Bridge) string {
	if bridge.APIBaseV1 != "" {
		return strings.TrimSuffix(bridge.APIBaseV1, "/")
	}
	base := normalizeBridgeAddress(bridge.Address)
	if bridge.Username != "" {
		usernameSuffix := "/api/" + bridge.Username
		if strings.HasSuffix(base, usernameSuffix) {
			return base
		}
		if strings.HasSuffix(base, "/api") {
			return base + "/" + bridge.Username
		}
		return base + usernameSuffix
	}
	if strings.HasSuffix(base, "/api") {
		return base
	}
	return base + "/api"
}

func normalizeBridgeAddress(address string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(address), "/")
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return "https://" + trimmed
}
