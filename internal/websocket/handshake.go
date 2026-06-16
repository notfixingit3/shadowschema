package websocket

import (
	"net/http"
	"strings"
)

// IsUpgrade reports whether the request is a WebSocket upgrade handshake.
func IsUpgrade(req *http.Request) bool {
	if req == nil {
		return false
	}
	return headerContains(req.Header, "Connection", "upgrade") &&
		headerContains(req.Header, "Upgrade", "websocket")
}

// IsUpgradeResponse reports whether the response completed a WebSocket upgrade.
func IsUpgradeResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusSwitchingProtocols &&
		headerContains(resp.Header, "Connection", "upgrade") &&
		headerContains(resp.Header, "Upgrade", "websocket")
}

// OpcodeName returns a human-readable WebSocket opcode label.
func OpcodeName(opcode byte) string {
	switch opcode {
	case 0x1:
		return "text"
	case 0x2:
		return "binary"
	case 0x8:
		return "close"
	case 0x9:
		return "ping"
	case 0xA:
		return "pong"
	default:
		return "unknown"
	}
}

func headerContains(header http.Header, name, value string) bool {
	for _, v := range header[name] {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(value, strings.TrimSpace(part)) {
				return true
			}
		}
	}
	return false
}