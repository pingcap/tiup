package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendCommandsAndPrintResult_FailedCommandDoesNotDuplicateErrorOutput(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CommandReply{
			OK:    false,
			Error: "boom",
		})
	}))
	defer s.Close()

	addr := strings.TrimPrefix(s.URL, "http://")

	var buf bytes.Buffer
	err := sendCommandsAndPrintResult(&buf, []Command{{Type: DisplayCommandType}}, addr)
	if err == nil {
		t.Fatalf("expected error")
	}
	printDisplayFailureWarning(&buf, err)

	out := buf.String()
	if got := strings.Count(out, "boom"); got != 1 {
		t.Fatalf("expected error text printed once, got %d:\n%s", got, out)
	}
}
