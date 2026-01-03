package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommandHandler_MethodNotAllowed(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodGet, "/command", nil)
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", w.Result().StatusCode)
	}
}

func TestCommandHandler_InvalidJSON(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader("{"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", w.Result().StatusCode, w.Body.String())
	}
	var reply CommandReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("unmarshal reply: %v (%q)", err, w.Body.String())
	}
	if reply.OK {
		t.Fatalf("unexpected ok reply: %+v", reply)
	}
	if reply.Error == "" {
		t.Fatalf("missing error reply: %+v", reply)
	}
}

func TestCommandHandler_UnknownFieldsRejected(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(`{"type":"display","unknown":1}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", w.Result().StatusCode, w.Body.String())
	}
	var reply CommandReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("unmarshal reply: %v (%q)", err, w.Body.String())
	}
	if reply.Error == "" {
		t.Fatalf("missing error reply: %+v", reply)
	}
}

func TestCommandHandler_TrailingDataRejected(t *testing.T) {
	p := &Playground{}
	body := `{"type":"display"}{"type":"display"}`
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", w.Result().StatusCode, w.Body.String())
	}
	var reply CommandReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("unmarshal reply: %v (%q)", err, w.Body.String())
	}
	if reply.Error != "invalid JSON payload" {
		t.Fatalf("unexpected error: %+v", reply)
	}
}

func TestCommandHandler_MaxBodyBytes(t *testing.T) {
	p := &Playground{}
	tooLarge := bytes.Repeat([]byte{'a'}, 1024*1024+1)
	r := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader(tooLarge))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", w.Result().StatusCode, w.Body.String())
	}
	var reply CommandReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("unmarshal reply: %v (%q)", err, w.Body.String())
	}
	if reply.Error == "" {
		t.Fatalf("missing error reply: %+v", reply)
	}
}
