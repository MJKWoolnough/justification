package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() error {
	port := flag.Uint("p", 8080, "port for server to listen on")
	dir := flag.String("d", "./schemas", "directory to store uplgoaded JSON schemas")
	flag.Parse()

	schema := NewSchema(*dir)

	if err := os.Mkdir(schema.Dir, 0o755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating schema directory: %w", err)
	}

	schemaDir, err := os.ReadDir(schema.Dir)
	if err != nil {
		return fmt.Errorf("error reading schema directory: %w", err)
	}

	for _, file := range schemaDir {
		name := file.Name()
		schemapath := filepath.Join(schema.Dir, name)
		f, err := os.Open(schemapath)
		if err != nil {
			return fmt.Errorf("error reading schema file (%s): %w", schemapath, err)
		}
		url := "schema://" + path.Join("/", schema.Dir, name)
		if err := schema.Compiler.AddResource(url, f); err != nil {
			return fmt.Errorf("error adding scheme as resource: %w", err)
		}
		s, err := schema.Compiler.Compile(url)
		if err != nil {
			return fmt.Errorf("error compiling schema: %w", err)
		}
		f.Close()
		schema.Schema[name] = s
	}

	server := &http.Server{
		Handler: schema,
	}

	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(*port)})
	if err != nil {
		return fmt.Errorf("error listening on port %d: %w", *port, err)
	}

	go server.Serve(l)

	// wait for SIGINT

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	<-sc
	signal.Stop(sc)
	close(sc)

	return server.Shutdown(context.Background())
}
