package main

import (
	"cmpserve/internal/service"
	"errors"
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	// Define optional parameters
	dir := flag.String("dir", ".", "Service directory")
	cacheDir := flag.String("cache-dir", ".", "Cache directory")
	addr := flag.String("addr", "0.0.0.0", "Bind address")
	port := flag.String("port", "8080", "Port number")
	createIndexes := flag.Bool("indexes", false, "Display indexes for directories")
	exposeHiddenFiles := flag.Bool("show-hidden-files", false, "Display and serve hidden files")

	flag.Parse()

	server, err := service.NewService(*dir, *cacheDir, *createIndexes, *exposeHiddenFiles)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	srv := &http.Server{
		Addr:         *addr + ":" + *port,
		Handler:      server,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Service running on %s:%s", *addr, *port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Service failed: %v", err)
	}
}
