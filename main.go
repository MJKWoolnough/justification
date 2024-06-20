package main // import "vimagination.zapto.org/justification"

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		log.Println(err)
	}
}

func run() error {
	defaultDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("error getting user config dir: %w", err)
	}

	port := flag.Uint("p", 8080, "port for server to listen on")
	dir := flag.String("d", filepath.Join(defaultDir, "justification"), "directory to store uploaded JSON schemas")

	flag.Parse()

	schema, err := NewSchema(*dir)
	if err != nil {
		return err
	}

	server := &http.Server{Handler: schema}

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
