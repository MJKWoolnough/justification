package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

type SchemaMap struct {
	Compiler *jsonschema.Compiler
	Dir      string

	mu     sync.RWMutex
	Schema map[string]*jsonschema.Schema
}

func NewSchema(dir string) *SchemaMap {
	return &SchemaMap{
		Compiler: jsonschema.NewCompiler(),
		Dir:      dir,
		Schema:   make(map[string]*jsonschema.Schema),
	}
}

func (s *SchemaMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/schema/") || r.URL.Path == "/schema" {
		s.handleSchema(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/validate/") || r.URL.Path == "/validate" {
		s.handleValidate(w, r)
	} else if r.URL.Path == "/" {
		// list endpoints
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

func (s *SchemaMap) handleSchema(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/schema")
	switch r.Method {
	case http.MethodGet:
		if id == "" || id == "/" {
			// list schema
		} else {
			s.serveSchema(w, r, id[1:])
		}
	case http.MethodPost:
		if id == "" || id == "/" {
			http.Error(w, "", http.StatusMethodNotAllowed)
		} else {
			s.uploadSchema(w, r, id[1:])
		}
	case http.MethodOptions:
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

func (s *SchemaMap) handleValidate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/schema")
	switch r.Method {
	case http.MethodGet:
		if id == "" || id == "/" {
			// list schema
		} else {
			http.Error(w, "", http.StatusMethodNotAllowed)
		}
	case http.MethodPost:
		if id == "" || id == "/" {
			http.Error(w, "", http.StatusMethodNotAllowed)
		} else {
			s.validateJSON(w, r, id[1:])
		}
	case http.MethodOptions:
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

func (s *SchemaMap) serveSchema(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.RLock()
	_, ok := s.Schema[id]
	s.mu.RUnlock()
	w.Header().Add("Content-Type", "application/json")
	if ok {
		http.ServeFile(w, r, filepath.Join(s.Dir, id))
	} else {
		http.Error(w, "", http.StatusNotFound)
	}
}

func (s *SchemaMap) uploadSchema(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Schema[id]; ok {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}
	var b bytes.Buffer
	io.Copy(&b, r.Body)
	if !json.Valid(b.Bytes()) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{
	"action": "uploadSchema",
	"id": %q,
	"status": "error",
	"message": "Invalid JSON"
}`, id)
		return
	}
	url := "schema://" + path.Join("/", s.Dir, id)
	if err := s.Compiler.AddResource(url, &b); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{
	"action": "uploadSchema",
	"id": %q,
	"status": "error",
	"message": %q
}`, id, err)
		return
	}
	cs, err := s.Compiler.Compile(url)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{
	"action": "uploadSchema",
	"id": %q,
	"status": "error",
	"message": %q
}`, id, err)
		return
	}
	s.Schema[id] = cs
	fmt.Fprintf(w, `{
	"action": "uploadSchema",
	"id": %q,
	"status": "success"
}`, id)
}

func (s *SchemaMap) validateJSON(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.RLock()
	schema, ok := s.Schema[id]
	s.mu.RUnlock()
	if ok {
		w.Header().Add("Content-Type", "application/json")
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{
	"action": "validateDocument",
	"id": %q,
	"status": "error",
	"message": "Invalid JSON"
}`, id)
		}
		removeNulls(v)
		if err := schema.Validate(v); err != nil {
			fmt.Fprintf(w, `{
	"action": "validateDocument",
	"id": %q,
	"status": "error",
	"message": %q
}`, id, err)
		} else {
			fmt.Fprintf(w, `{
	"action": "validateDocument",
	"id": %q,
	"status": "success"
}`, id)
		}
	} else {
		http.Error(w, "", http.StatusNotFound)
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
