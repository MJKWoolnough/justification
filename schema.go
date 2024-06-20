package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	optionsPost    = "OPTIONS, POST"
	optionsGetHead = "OPTIONS, GET, HEAD"
)

func validID(id string) bool {
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && c != '_' && c != '-' {
			return false
		}
	}

	return true
}

func respond(w http.ResponseWriter, code int, format string, a ...interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, format, a...)
}

type Schema struct {
	Compiler *jsonschema.Compiler
	Dir      string

	mu     sync.RWMutex
	Schema map[string]*jsonschema.Schema
}

func NewSchema(dir string) (*Schema, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("error creating schema directory: %w", err)
	}

	schemaDir, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error reading schema directory: %w", err)
	}

	c := jsonschema.NewCompiler()
	m := make(map[string]*jsonschema.Schema)

	for _, file := range schemaDir {
		name := file.Name()
		schemapath := filepath.Join(dir, name)

		f, err := os.Open(schemapath)
		if err != nil {
			return nil, fmt.Errorf("error reading schema file (%s): %w", schemapath, err)
		}

		url := "schema:///" + name

		if rerr := c.AddResource(url, f); rerr != nil {
			return nil, fmt.Errorf("error adding scheme as resource: %w", rerr)
		}

		s, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("error compiling schema: %w", err)
		}

		f.Close()

		m[name] = s
	}

	return &Schema{
		Compiler: c,
		Dir:      dir,
		Schema:   m,
	}, nil
}

func (s *Schema) hasID(id string) bool {
	s.mu.RLock()
	_, ok := s.Schema[id]
	s.mu.RUnlock()

	return ok
}

func (s *Schema) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/schema/") {
		s.handleSchema(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/validate/") {
		s.handleValidate(w, r)
	} else {
		respond(w, http.StatusNotFound, `{"status": "error", "message": Unknown Endpoint" }`)
	}
}

func (s *Schema) handleSchema(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/schema/")
	if !validID(id) {
		respond(w, http.StatusBadRequest, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Invalid ID"}`, id)

		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		s.serveSchema(w, r, id)
	case http.MethodPost:
		s.uploadSchema(w, r, id)
	default:
		if s.hasID(id) {
			w.Header().Add("Allow", optionsGetHead)
		} else {
			w.Header().Add("Allow", optionsPost)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
		} else {
			respond(w, http.StatusMethodNotAllowed, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Method Not Allowed"}`, id)
		}
	}
}

func (s *Schema) handleValidate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/validate/")
	if !validID(id) {
		respond(w, http.StatusBadRequest, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Invalid ID"}`, id)

		return
	}

	if s.hasID(id) {
		switch r.Method {
		case http.MethodPost:
			s.validateJSON(w, r, id)
		case http.MethodOptions:
			w.Header().Add("Allow", optionsPost)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Add("Allow", optionsPost)
			respond(w, http.StatusMethodNotAllowed, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Method Not Allowed"}`, id)
		}
	} else {
		respond(w, http.StatusNotFound, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Unknown ID"}`, id)
	}
}

func (s *Schema) serveSchema(w http.ResponseWriter, r *http.Request, id string) {
	if s.hasID(id) {
		w.Header().Set("Content-Type", "application/schema+json")
		http.ServeFile(w, r, filepath.Join(s.Dir, id))
	} else {
		respond(w, http.StatusMethodNotAllowed, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Method Not Allowed"}`, id)
	}
}

func (s *Schema) uploadSchema(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Schema[id]; ok {
		w.Header().Add("Allow", optionsGetHead)
		respond(w, http.StatusMethodNotAllowed, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Method Not Allowed"}`, id)

		return
	}

	var b bytes.Buffer

	io.Copy(&b, r.Body)

	data := b.Bytes()
	url := "schema:///" + id

	if err := s.Compiler.AddResource(url, &b); err != nil {
		respond(w, http.StatusBadRequest, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Invalid JSON"}`, id)

		return
	}

	cs, err := s.Compiler.Compile(url)
	if err != nil {
		respond(w, http.StatusBadRequest, `{"action": "uploadSchema", "id": %q, "status": "error", "message": %q}`, id, err)

		return
	}

	f, err := os.Create(filepath.Join(s.Dir, id))
	if err == nil {
		if _, err = f.Write(data); err == nil {
			if err = f.Close(); err == nil {
				s.Schema[id] = cs

				respond(w, http.StatusCreated, `{"action": "uploadSchema", "id": %q, "status": "success"}`, id)

				return
			}
		}
	}

	respond(w, http.StatusInternalServerError, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Unexpected Error"}`, id)
	log.Printf("error saving schema: %s", err)
}

func (s *Schema) validateJSON(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.RLock()
	schema := s.Schema[id]
	s.mu.RUnlock()

	dec := json.NewDecoder(r.Body)

	dec.UseNumber()

	var v interface{}

	if err := dec.Decode(&v); err != nil {
		respond(w, http.StatusBadRequest, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Invalid JSON"}`, id)

		return
	}

	removeNulls(v)

	if err := schema.Validate(v); err != nil {
		respond(w, http.StatusOK, `{"action": "validateDocument", "id": %q, "status": "error", "message": %q}`, id, err)
	} else {
		respond(w, http.StatusOK, `{"action": "validateDocument", "id": %q, "status": "success"}`, id)
	}
}

func removeNulls(v interface{}) {
	if obj, ok := v.(map[string]interface{}); ok {
		for key, value := range obj {
			if value == nil {
				delete(obj, key)
			} else {
				removeNulls(value)
			}
		}
	}
}
