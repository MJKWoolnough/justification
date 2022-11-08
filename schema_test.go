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
			Message: "Method Not Allowed",
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
		} else if test.Message != "Invalid ID" {
			resp, err := http.Get(server.URL + "/schema/" + test.ID)
			var (
				expectingCode int
				expectingJSON string
			)
			if test.Code == http.StatusCreated || test.Code == http.StatusMethodNotAllowed {
				expectingCode = http.StatusOK
				expectingJSON = test.JSON
			} else {
				expectingCode = http.StatusMethodNotAllowed
				expectingJSON = fmt.Sprintf(`{"action": "uploadSchema", "id": %q, "status": "error", "message": "Method Not Allowed"}`, test.ID)
			}
			var b bytes.Buffer
			if err != nil {
				t.Errorf("test %d.2: unexpected error grabbing Scheme JSON: %s", n+1, err)
			} else if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("test %d.2: expecting Content-Type of \"application/json\", %s", n+1, ct)
			} else if resp.StatusCode != expectingCode {
				t.Errorf("test %d.2: expecting status code %d, got %d", n+1, expectingCode, resp.StatusCode)
			} else if _, err := io.Copy(&b, resp.Body); err != nil {
				t.Errorf("test %d.2: unexpected error reading Scheme JSON: %s", n+1, err)
			} else if str := b.String(); str != expectingJSON {
				t.Errorf("test %d.2: expecting to read JSON %q, got %q", n+1, expectingJSON, str)
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

func TestValidate(t *testing.T) {
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
	for n, test := range [...]struct {
		ID, JSON, Status, Message string
		Code                      int
	}{
		{
			ID:     "SimpleBoolean",
			JSON:   "{}",
			Status: "success",
			Code:   http.StatusOK,
		},
		{
			ID:      "Bad ID",
			JSON:    "{}",
			Status:  "error",
			Message: "Invalid ID",
			Code:    http.StatusBadRequest,
		},
		{
			ID:      "Unknown",
			JSON:    "{}",
			Status:  "error",
			Message: "Unknown ID",
			Code:    http.StatusNotFound,
		},
		{
			ID:      "SimpleBoolean",
			JSON:    "{",
			Status:  "error",
			Message: "Invalid JSON",
			Code:    http.StatusBadRequest,
		},
		{
			ID:     "SimpleObject",
			JSON:   "{}",
			Status: "success",
			Code:   http.StatusOK,
		},
		{
			ID:      "Complex",
			JSON:    "{}",
			Status:  "error",
			Message: "jsonschema: '' does not validate with schema:///Complex#/required: missing properties: 'source', 'destination'",
			Code:    http.StatusOK,
		},
		{
			ID:     "Complex",
			JSON:   `{"source": "/home/alice/image.iso", "destination": "/mnt/storage", "chunks": {"size": 1024}}`,
			Status: "success",
			Code:   http.StatusOK,
		},
		{
			ID:     "Complex",
			JSON:   `{"source": "/home/alice/image.iso", "destination": "/mnt/storage", "timeout": null, "chunks": {"size": 1024, "number": null}}`,
			Status: "success",
			Code:   http.StatusOK,
		},
		{
			ID:      "Complex",
			JSON:    `{"source": "/home/alice/image.iso", "destination": null}`,
			Status:  "error",
			Message: "jsonschema: '' does not validate with schema:///Complex#/required: missing properties: 'destination'",
			Code:    http.StatusOK,
		},
	} {
		resp, err := http.Post(server.URL+"/validate/"+test.ID, "application/json", strings.NewReader(test.JSON))
		var r response
		if err != nil {
			t.Errorf("test %d: unexpected error: %s", n+1, err)
		} else if resp.StatusCode != test.Code {
			t.Errorf("test %d: expecting status code %d, got %d", n+1, test.Code, resp.StatusCode)
		} else if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("test %d: expecting Content-Type of \"application/json\", %s", n+1, ct)
		} else if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Errorf("test %d: unexpected error: %s", n+1, err)
		} else if r.Action != "validateDocument" {
			t.Errorf("test %d: expecting Action \"validateDocument\", got %q", n+1, r.Action)
		} else if r.ID != test.ID {
			t.Errorf("test %d: expecting ID %q, got %q", n+1, test.ID, r.ID)
		} else if r.Status != test.Status {
			t.Errorf("test %d: expecting Status %q, got %q", n+1, test.Status, r.Status)
		} else if r.Message != test.Message {
			t.Errorf("test %d: expecting Message %q, got %q", n+1, test.Message, r.Message)
		}
	}
}

func TestOptions(t *testing.T) {
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
	var c http.Client
	for n, test := range [...]struct {
		Endpoint, Options string
		Code              int
	}{
		{
			Endpoint: "/schema/Unknown",
			Options:  optionsPost,
			Code:     http.StatusNoContent,
		},
		{
			Endpoint: "/schema/Complex",
			Options:  optionsGetHead,
			Code:     http.StatusNoContent,
		},
		{
			Endpoint: "/validate/Unknown",
			Options:  "",
			Code:     http.StatusNotFound,
		},
		{
			Endpoint: "/validate/Complex",
			Options:  optionsPost,
			Code:     http.StatusNoContent,
		},
		{
			Endpoint: "/other-endpoint",
			Options:  "",
			Code:     http.StatusNotFound,
		},
	} {
		req, _ := http.NewRequest("OPTIONS", server.URL+test.Endpoint, nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Errorf("test %d: unexpected error: %s", n+1, err)
		} else if resp.StatusCode != test.Code {
			t.Errorf("test %d: expecting status code %d, got %d", n+1, test.Code, resp.StatusCode)
		} else if options := resp.Header.Get("Allow"); options != test.Options {
			t.Errorf("test %d: expecting options %q, got %q", n+1, test.Options, options)
		}
	}
}
