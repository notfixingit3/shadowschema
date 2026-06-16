package websocket

import (
	"net/http"
	"testing"
)

func TestIsUpgrade(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	if !IsUpgrade(req) {
		t.Fatalf("expected websocket upgrade request to be detected")
	}

	req.Header.Del("Upgrade")
	if IsUpgrade(req) {
		t.Fatalf("expected non-upgrade request to be rejected")
	}
}

func TestIsUpgradeResponse(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusSwitchingProtocols,
		Header: http.Header{
			"Connection": []string{"Upgrade"},
			"Upgrade":    []string{"websocket"},
		},
	}

	if !IsUpgradeResponse(resp) {
		t.Fatalf("expected 101 websocket response to be detected")
	}

	resp.StatusCode = http.StatusOK
	if IsUpgradeResponse(resp) {
		t.Fatalf("expected non-101 response to be rejected")
	}
}