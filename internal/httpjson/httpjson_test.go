package httpjson

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeAcceptsOneJSONValue(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	var body struct {
		Name string `json:"name"`
	}
	if err := Decode(recorder, request, &body); err != nil {
		t.Fatal(err)
	}
	if body.Name != "alice" {
		t.Fatalf("name = %q", body.Name)
	}
}

func TestDecodeRejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	tests := []string{
		`{"name":"alice","admin":true}`,
		`{"name":"alice"} {"name":"bob"}`,
	}
	for _, raw := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/", strings.NewReader(raw))
		request.Header.Set("Content-Type", "application/json")
		var body struct {
			Name string `json:"name"`
		}
		if err := Decode(recorder, request, &body); err == nil {
			t.Fatalf("invalid body %q was accepted", raw)
		}
	}
}

func TestDecodeRequiresJSONContentType(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"}`))
	request.Header.Set("Content-Type", "text/plain")
	var body map[string]string
	if err := Decode(recorder, request, &body); err == nil {
		t.Fatal("non-JSON content type was accepted")
	}
}
