package spec

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleHAR(url string) string {
	return `{
  "log": {
    "version": "1.2",
    "entries": [
      {
        "request": {
          "method": "POST",
          "url": "` + url + `/api/items",
          "headers": [
            {"name": "Authorization", "value": "Bearer har-token"},
            {"name": "Content-Type", "value": "application/json"}
          ],
          "queryString": [],
          "postData": {"mimeType": "application/json", "text": "{\"name\":\"from-har\"}"}
        },
        "response": {
          "status": 201,
          "headers": [],
          "content": {"text": "{\"id\":7,\"name\":\"from-har\"}", "mimeType": "application/json"}
        }
      },
      {
        "request": {
          "method": "GET",
          "url": "https://cdn.other.com/static.js",
          "headers": [],
          "queryString": []
        },
        "response": {
          "status": 200,
          "headers": [],
          "content": {"text": "console.log(1)"}
        }
      }
    ]
  }
}`
}

func TestImportHARMapsMatchingEntries(t *testing.T) {
	sm := newTestSpecManager(t, "api.example.com")
	har := sampleHAR("https://api.example.com")

	result, err := sm.ImportHAR(strings.NewReader(har), true)
	if err != nil {
		t.Fatalf("ImportHAR failed: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("expected 1 imported, got %+v", result)
	}
	if result.Skipped < 1 {
		t.Fatalf("expected out-of-target entry skipped, got %+v", result)
	}

	pathItem := sm.doc.Paths.Find("/api/items")
	if pathItem == nil || pathItem.Post == nil {
		t.Fatal("expected POST /api/items from HAR")
	}
	if pathItem.Post.Responses.Value("201") == nil {
		t.Fatal("expected 201 from HAR")
	}
	reqBody, ok := pathItem.Post.Extensions["x-last-request-body"].(map[string]interface{})
	if !ok || reqBody["name"] != "from-har" {
		t.Fatalf("expected request body from HAR, got %#v", pathItem.Post.Extensions["x-last-request-body"])
	}

	creds, err := sm.listVaultCredentials()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range creds {
		if c.HeaderName == "Authorization" && c.TokenValue == "Bearer har-token" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected vault credential from HAR, got %#v", creds)
	}
}

func TestImportHARHTTPEndpoint(t *testing.T) {
	sm := newTestSpecManager(t, "api.example.com")
	server := httptest.NewServer(sm.ExportHandler())
	defer server.Close()

	body := sampleHAR("https://api.example.com")
	resp, err := http.Post(server.URL+"/import-har", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result HARImportResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Imported != 1 {
		t.Fatalf("expected imported=1, got %+v", result)
	}
}
