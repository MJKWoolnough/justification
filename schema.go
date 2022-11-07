package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

type SchemaMap struct {
	Dir string

	mu     sync.RWMutex
	Schema map[string]*jsonschema.Schema
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
}

func (s *SchemaMap) validateJSON(w http.ResponseWriter, r *http.Request, id string) {
}
