package main

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Server struct {
	archiveDir string
}

func NewServer(archiveDir string) (*Server, error) {
	return &Server{
		archiveDir: archiveDir,
	}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(urlPath, "/")

	currentPath := s.archiveDir
	var archivePath string
	var remainingPath string

	for i, part := range parts {
		currentPath = filepath.Join(currentPath, part)

		if stat, err := os.Stat(currentPath); err == nil {
			if stat.IsDir() {
				continue
			} else {
				http.ServeFile(w, r, currentPath)
				return
			}
		}

		for _, ext := range []string{".zip", ".tar", ".tar.bz"} {
			archiveCandidate := currentPath + ext
			if _, err := os.Stat(archiveCandidate); err == nil {
				archivePath = archiveCandidate
				if len(parts)-1 == i {
					http.Redirect(w, r, "/"+urlPath+"/", http.StatusMovedPermanently)
					return
				} else {
					remainingPath = strings.Join(parts[i+1:], "/")
				}
				break
			}
		}

		if archivePath != "" {
			break
		}

		http.NotFound(w, r)
		return
	}

	if archivePath == "" {
		http.NotFound(w, r)
		return
	}

	if remainingPath == "" {
		remainingPath = "index.html"
	}

	// Serve the file from the zip archive
	s.serveFileFromZip(w, r, archivePath, remainingPath)
}

func (s *Server) serveFileFromArchive(w http.ResponseWriter, r *http.Request, archivePath, filePath string) {
	filePath, err := url.PathUnescape(filePath)
	if err != nil {
		http.Error(w, "Invalid file path encoding", http.StatusBadRequest)
		return
	}

	switch filepath.Ext(archivePath) {
	case ".zip":
		s.serveFileFromZip(w, r, archivePath, filePath)
	case ".tar", ".tar.bz":
		s.serveFileFromTar(w, r, archivePath, filePath)
	default:
		http.Error(w, "Unsupported archive format", http.StatusUnsupportedMediaType)
	}
}
func (s *Server) serveFileFromZip(w http.ResponseWriter, r *http.Request, zipPath, filePath string) {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		http.Error(w, "Failed to open archive", http.StatusInternalServerError)
		return
	}
	defer func(zipReader *zip.ReadCloser) {
		_ = zipReader.Close()
	}(zipReader)

	var foundFile *zip.File
	for _, file := range zipReader.File {
		if file.Name == filePath {
			foundFile = file
		}
	}

	if foundFile != nil {
		reader, err := foundFile.Open()
		if err != nil {
			http.Error(w, "Failed to open file in archive", http.StatusInternalServerError)
			return
		}
		defer func(reader io.ReadCloser) {
			_ = reader.Close()
		}(reader)

		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(foundFile.Name)))
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, reader)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) serveFileFromTar(w http.ResponseWriter, r *http.Request, tarPath, filePath string) {
	file, err := os.Open(tarPath)
	if err != nil {
		http.Error(w, "Failed to open archive", http.StatusInternalServerError)
		return
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	tarReader := tar.NewReader(file)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Failed to read archive", http.StatusInternalServerError)
			return
		}

		if hdr.Name == filePath {
			w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(hdr.Name)))
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, tarReader)
			return
		}
	}

	http.NotFound(w, r)
}

func main() {
	server, err := NewServer(".")
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      server,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Println("Server running on :8080")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server failed: %v", err)
	}
}
