package main

import (
	"cmpserve/internal/service"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"
)

// getEnvWithDefault fetches an environment variable or falls back to a default value
func getEnvWithDefault(envKey, defaultValue string) string {
	if val, exists := os.LookupEnv(envKey); exists {
		return val
	}
	return defaultValue
}

func main() {
	dir := flag.String("dir", getEnvWithDefault("CMPSERVE_DIR", "."), "Service directory")
	cacheDir := flag.String("cache-dir", getEnvWithDefault("CMPSERVE_CACHE_DIR", "."), "Cache directory")
	addr := flag.String("addr", getEnvWithDefault("CMPSERVE_ADDR", "0.0.0.0"), "Bind address")
	port := flag.String("port", getEnvWithDefault("CMPSERVE_PORT", "8080"), "Port number")
	createIndexes := flag.Bool("indexes", os.Getenv("CMPSERVE_INDEXES") == "true", "Display indexes for directories")
	exposeHiddenFiles := flag.Bool("show-hidden-files", os.Getenv("CMPSERVE_SHOW_HIDDEN_FILES") == "true", "Display and serve hidden files")

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
