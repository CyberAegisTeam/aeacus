package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigImportReturnsDesktopSafeCollections(t *testing.T) {
	server := &studioServer{mode: "personal", token: "test-token", project: newProject()}
	content := `
name = "Linux_PR6"
title = "Linux Practice Round 6 [2026]"
os = "Linux Mint 22"
user = "red"

[[check]]
message = "Removed unauthorized file"
points = 5
  [[check.pass]]
  type = "PathExistsNot"
  path = "/tmp/unauthorized.mp3"
`
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Aeacus-Token", "test-token")
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import returned %d: %s", response.Code, response.Body.String())
	}
	for _, nullCollection := range []string{`"passOverride":null`, `"fail":null`, `"forensics":null`, `"users":null`} {
		if strings.Contains(response.Body.String(), nullCollection) {
			t.Fatalf("import response contains unsafe collection %s: %s", nullCollection, response.Body.String())
		}
	}
}
