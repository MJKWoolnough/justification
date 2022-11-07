package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, format, a...)
}

type SchemaMap struct {
	Compiler *jsonschema.Compiler
	Dir      string

	mu     sync.RWMutex
	Schema map[string]*jsonschema.Schema
}

func NewSchema(dir string) (*SchemaMap, error) {
	if err := os.Mkdir(dir, 0o755); err != nil && !os.IsExist(err) {
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
		if err := c.AddResource(url, f); err != nil {
			return nil, fmt.Errorf("error adding scheme as resource: %w", err)
		}
		s, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("error compiling schema: %w", err)
		}
		f.Close()
		m[name] = s
	}
	return &SchemaMap{
		Compiler: c,
		Dir:      dir,
		Schema:   m,
	}, nil
}

func (s *SchemaMap) hasID(id string) bool {
	s.mu.RLock()
	_, ok := s.Schema[id]
	s.mu.RUnlock()
	return ok
}

func (s *SchemaMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/schema/") {
		s.handleSchema(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/validate/") {
		s.handleValidate(w, r)
	} else {
		respond(w, http.StatusNotFound, `{"status": "error", "message": Unknown Endpoint" }`)
	}
}

func (s *SchemaMap) handleSchema(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/schema/")
	if !validID(id) {
		respond(w, http.StatusBadRequest, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Invalid ID"}`, id)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.serveSchema(w, r, id)
	case http.MethodPost:
		s.uploadSchema(w, r, id)
	case http.MethodOptions:
		s.handleSchemaOptions(w, r, id)
	default:
		respond(w, http.StatusMethodNotAllowed, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Method Not Allowed"}`, id)
	}
}

func (s *SchemaMap) handleValidate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/validate/")
	if !validID(id) {
		respond(w, http.StatusBadRequest, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Invalid ID"}`, id)
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.validateJSON(w, r, id)
	case http.MethodOptions:
		s.handleValidateOptions(w, r, id)
	default:
		respond(w, http.StatusMethodNotAllowed, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Method Not Allowed"}`, id)
	}
}

func (s *SchemaMap) handleSchemaOptions(w http.ResponseWriter, r *http.Request, id string) {
	if s.hasID(id) {
		w.Header().Add("Allow", optionsGetHead)
	} else {
		w.Header().Add("Allow", optionsPost)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *SchemaMap) handleValidateOptions(w http.ResponseWriter, r *http.Request, id string) {
	if s.hasID(id) {
		w.Header().Add("Allow", optionsPost)
		w.WriteHeader(http.StatusNoContent)
	} else {
		respond(w, http.StatusMethodNotAllowed, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Method Not Allowed"}`, id)
	}
}

func (s *SchemaMap) serveSchema(w http.ResponseWriter, r *http.Request, id string) {
	if s.hasID(id) {
		http.ServeFile(w, r, filepath.Join(s.Dir, id))
	} else {
		respond(w, http.StatusNotFound, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "Unknown ID"}`, id)
	}
}

func (s *SchemaMap) uploadSchema(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Schema[id]; ok {
		respond(w, http.StatusMethodNotAllowed, `{"action": "uploadSchema", "id": %q, "status": "error", "message": "ID Exists"}`, id)
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
}

func (s *SchemaMap) validateJSON(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.RLock()
	schema, ok := s.Schema[id]
	s.mu.RUnlock()
	if ok {
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			respond(w, http.StatusBadRequest, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Invalid JSON"}`, id)
		}
		removeNulls(v)
		if err := schema.Validate(v); err != nil {
			respond(w, http.StatusBadRequest, `{"action": "validateDocument", "id": %q, "status": "error", "message": %q}`, id, err)
		} else {
			respond(w, http.StatusOK, `{"action": "validateDocument", "id": %q, "status": "success"}`, id)
		}
	} else {
		respond(w, http.StatusNotFound, `{"action": "validateDocument", "id": %q, "status": "error", "message": "Unknown ID"}`, id)
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
