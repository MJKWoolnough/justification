package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
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

	schema, err := NewSchema(*dir)
	if err != nil {
		return err
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
