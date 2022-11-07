package main

import (
	"net/http"
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
	}
}

func (s *SchemaMap) handleSchema(w http.ResponseWriter, r *http.Request) {
}
