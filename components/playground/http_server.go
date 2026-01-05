package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (p *Playground) listenAndServeHTTP() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/command", p.commandHandler)

	srv := &http.Server{
		Addr:              "127.0.0.1:" + strconv.Itoa(p.port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}

	go func() {
		if p == nil || p.processGroup == nil {
			return
		}
		<-p.processGroup.Closed()
		_ = srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (p *Playground) commandHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
		return
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "content-type must be application/json"})
		return
	}

	var cmd Command
	const maxBodyBytes = 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&cmd)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: err.Error()})
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "invalid JSON payload"})
		return
	}

	output, err := p.doCommand(r.Context(), &cmd)

	reply := CommandReply{OK: err == nil, Message: string(output)}
	if err != nil {
		reply.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(w).Encode(&reply)
}
