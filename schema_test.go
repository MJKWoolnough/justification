package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type response struct {
	Action, ID, Status, Message string
}

func TestUpload(t *testing.T) {
	dir, err := os.MkdirTemp("", "justification")
	if err != nil {
		t.Errorf("unexepected error creating Schema: %s", err)
		return
	}
	defer os.RemoveAll(dir)
	s, err := NewSchema(dir)
	if err != nil {
		t.Errorf("unexepected error creating Schema: %s", err)
		return
	}
	server := httptest.NewServer(s)
	defer server.Close()
	for n, test := range [...]struct {
		ID, JSON, Status, Message string
		Code                      int
	}{
		{
			ID:      "TEST",
			JSON:    ``,
			Code:    http.StatusBadRequest,
			Status:  "error",
			Message: "Invalid JSON",
		},
		{
			ID:     "TEST",
			JSON:   `{}`,
			Code:   http.StatusCreated,
			Status: "success",
		},
		{
			ID:      "TEST",
			JSON:    `{}`,
			Code:    http.StatusMethodNotAllowed,
			Status:  "error",
			Message: "ID Exists",
		},
		{
			ID:      "ANOTHER_TEST",
			JSON:    `{"$schema": "some invalid schema"}`,
			Code:    http.StatusBadRequest,
			Status:  "error",
			Message: "jsonschema schema:///ANOTHER_TEST compilation failed: invalid $schema in schema:///ANOTHER_TEST",
		},
		{
			ID:      "BAD ID",
			JSON:    `{}`,
			Code:    http.StatusBadRequest,
			Status:  "error",
			Message: "Invalid ID",
		},
		{
			ID:     "FULL-SCHEMA",
			JSON:   `{"$schema": "http://json-schema.org/draft-04/schema#", "type": "object", "properties": {"source": {"type": "string"}, "destination": {"type": "string"}, "timeout": {"type": "integer", "minimum": 0, "maximum": 32767}, "chunks": {"type": "object", "properties": {"size": {"type": "integer"}, "number": {"type": "integer"}}, "required": ["size"]}}, "required": ["source", "destination"]}`,
			Code:   http.StatusCreated,
			Status: "success",
		},
	} {
		resp, err := http.Post(server.URL+"/schema/"+test.ID, "application/json", strings.NewReader(test.JSON))
		var r response
		if err != nil {
			t.Errorf("test %d.1: unexpected error: %s", n+1, err)
		} else if resp.StatusCode != test.Code {
			t.Errorf("test %d.1: expecting status code %d, got %d", n+1, test.Code, resp.StatusCode)
		} else if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("test %d.1: expecting Content-Type of \"application/json\", %s", n+1, ct)
		} else if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Errorf("test %d.1: unexpected error: %s", n+1, err)
		} else if r.Action != "uploadSchema" {
			t.Errorf("test %d.1: expecting Action \"uploadSchema\", got %q", n+1, r.Action)
		} else if r.ID != test.ID {
			t.Errorf("test %d.1: expecting ID %q, got %q", n+1, test.ID, r.ID)
		} else if r.Status != test.Status {
			t.Errorf("test %d.1: expecting Status %q, got %q", n+1, test.Status, r.Status)
		} else if r.Message != test.Message {
			t.Errorf("test %d.1: expecting Message %q, got %q", n+1, test.Message, r.Message)
		} else if test.Code == http.StatusCreated {
			resp, err := http.Get(server.URL + "/schema/" + test.ID)
			var b bytes.Buffer
			if err != nil {
				t.Errorf("test %d.2: unexpected error grabbing Scheme JSON: %s", n+1, err)
			} else if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("test %d.2: expecting Content-Type of \"application/json\", %s", n+1, ct)
			} else if resp.StatusCode != http.StatusOK {
				t.Errorf("test %d.2: expecting status code 200, got %d", n+1, resp.StatusCode)
			} else if _, err := io.Copy(&b, resp.Body); err != nil {
				t.Errorf("test %d.2: unexpected error reading Scheme JSON: %s", n+1, err)
			} else if str := b.String(); str != test.JSON {
				t.Errorf("test %d.2: expecting to read JSON %q, got %q", n+1, test.JSON, str)
			}
		}
	}
}

type IDSchema struct {
	ID, JSON string
}

var schemaTestJSON = []IDSchema{
	{
		ID:   "SimpleBoolean",
		JSON: "true",
	},
	{
		ID:   "SimpleObject",
		JSON: "{}",
	},
	{
		ID:   "Complex",
		JSON: `{"$schema": "http://json-schema.org/draft-04/schema#", "type": "object", "properties": {"source": {"type": "string"}, "destination": {"type": "string"}, "timeout": {"type": "integer", "minimum": 0, "maximum": 32767}, "chunks": {"type": "object", "properties": {"size": {"type": "integer"}, "number": {"type": "integer"}}, "required": ["size"]}}, "required": ["source", "destination"]}`,
	},
}

func insertSchemas(url string, schemas []IDSchema) error {
	for n, test := range schemas {
		resp, err := http.Post(url+"/schema/"+test.ID, "application/json", strings.NewReader(test.JSON))
		var r response
		if err != nil {
			return fmt.Errorf("test %d: unexpected error: %w", n+1, err)
		} else if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("test %d: unexpected error, expecting status code 201, got %d", n+1, resp.StatusCode)
		} else if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return fmt.Errorf("test %d: unexpected error: %w", n+1, err)
		} else if r.Status != "success" {
			return fmt.Errorf("test %d: unexpected error, expecting Message \"success\", got %q", n+1, r.Status)
		}
	}
	return nil
}

func TestLoad(t *testing.T) {
	dir, err := os.MkdirTemp("", "justification")
	if err != nil {
		t.Errorf("unexepected error creating Schema: %s", err)
		return
	}
	defer os.RemoveAll(dir)
	s, err := NewSchema(dir)
	if err != nil {
		t.Errorf("unexepected error creating Schema: %s", err)
		return
	}
	server := httptest.NewServer(s)
	defer server.Close()
	if err = insertSchemas(server.URL, schemaTestJSON); err != nil {
		t.Error(err)
		return
	}
	s, err = NewSchema(dir)
	if err != nil {
		t.Errorf("unexepected error loading Schema: %s", err)
		return
	}
	server = httptest.NewServer(s)
	defer server.Close()
	for n, test := range schemaTestJSON {
		resp, err := http.Get(server.URL + "/schema/" + test.ID)
		var b bytes.Buffer
		if err != nil {
			t.Errorf("test %d: unexpected error grabbing Scheme JSON: %s", n+1, err)
		} else if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("test %d: expecting Content-Type of \"application/json\", %s", n+1, ct)
		} else if resp.StatusCode != http.StatusOK {
			t.Errorf("test %d: expecting status code 200, got %d", n+1, resp.StatusCode)
		} else if _, err := io.Copy(&b, resp.Body); err != nil {
			t.Errorf("test %d: unexpected error reading Scheme JSON: %s", n+1, err)
		} else if str := b.String(); str != test.JSON {
			t.Errorf("test %d: expecting to read JSON %q, got %q", n+1, test.JSON, str)
		}
	}
}
