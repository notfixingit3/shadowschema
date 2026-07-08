package spec

import (
	"net/http"
	"testing"
)

func TestAddEndpointIndexesGraphQLOperations(t *testing.T) {
	sm := newTestSpecManager(t, "api.example.com")
	req, _ := http.NewRequest(http.MethodPost, "http://api.example.com/graphql", nil)
	requestBody := []byte(`{
		"operationName": "GetUser",
		"query": "query GetUser($id: ID!) { user(id: $id) { id name } }",
		"variables": {"id": "1"}
	}`)
	responseBody := []byte(`{"data":{"user":{"id":"1","name":"Ada"}}}`)

	sm.AddEndpoint(req, "/graphql", 200, responseBody, requestBody)

	pathItem := sm.doc.Paths.Find("/graphql")
	if pathItem == nil || pathItem.Post == nil {
		t.Fatal("expected POST /graphql")
	}
	if pathItem.Post.Extensions["x-graphql"] != true {
		t.Fatal("expected x-graphql flag")
	}
	ops, ok := pathItem.Post.Extensions["x-graphql-operations"].(map[string]interface{})
	if !ok || len(ops) == 0 {
		t.Fatalf("expected x-graphql-operations, got %#v", pathItem.Post.Extensions["x-graphql-operations"])
	}
	entry, ok := ops["query:GetUser"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected query:GetUser entry, got %#v", ops)
	}
	if entry["type"] != "query" {
		t.Fatalf("expected type query, got %#v", entry["type"])
	}
	if entry["operationName"] != "GetUser" {
		t.Fatalf("expected operationName GetUser, got %#v", entry["operationName"])
	}
}

func TestLooksLikeGraphQLByPath(t *testing.T) {
	if !looksLikeGraphQL("/api/gql", []byte(`{}`)) {
		t.Fatal("expected path hint to match /api/gql")
	}
	if looksLikeGraphQL("/api/users", []byte(`{"name":"x"}`)) {
		t.Fatal("plain JSON should not look like GraphQL")
	}
}
