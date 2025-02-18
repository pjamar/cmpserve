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
	archiveDir = filepath.Clean(archiveDir)
	if stat, err := os.Stat(archiveDir); err != nil || !stat.IsDir() {
		return nil, errors.New("invalid archive directory")
	}
	return &Server{archiveDir: archiveDir}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(urlPath, "/")

	currentPath := s.archiveDir
	var archivePath, remainingPath string

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
				if i == len(parts)-1 {
					http.Redirect(w, r, "/"+urlPath+"/", http.StatusMovedPermanently)
					return
				}
				remainingPath = strings.Join(parts[i+1:], "/")
				break
			}
		}

		if archivePath != "" {
			break
		}
	}

	if archivePath == "" {
		http.NotFound(w, r)
		return
	}

	if remainingPath == "" {
		remainingPath = "index.html"
	}

	s.serveFileFromArchive(w, r, archivePath, remainingPath)
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
		log.Printf("Error opening ZIP archive: %v", err)
		return
	}
	defer zipReader.Close()

	filePath = filepath.ToSlash(filePath)

	zipMap := make(map[string]*zip.File)
	for _, file := range zipReader.File {
		zipMap[file.Name] = file
	}

	foundFile, exists := zipMap[filePath]
	if !exists {
		http.NotFound(w, r)
		return
	}

	reader, err := foundFile.Open()
	if err != nil {
		http.Error(w, "Failed to open file in archive", http.StatusInternalServerError)
		log.Printf("Error opening file %s in ZIP archive: %v", filePath, err)
		return
	}
	defer func(reader io.ReadCloser) {
		_ = reader.Close()
	}(reader)

	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(foundFile.Name)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

func (s *Server) serveFileFromTar(w http.ResponseWriter, r *http.Request, tarPath, filePath string) {
	file, err := os.Open(tarPath)
	if err != nil {
		http.Error(w, "Failed to open archive", http.StatusInternalServerError)
		log.Printf("Error opening TAR archive: %v", err)
		return
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	tarReader := tar.NewReader(file)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "Failed to read archive", http.StatusInternalServerError)
			log.Printf("Error reading TAR archive: %v", err)
			return
		}

		if hdr.Name == filePath {
			w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(hdr.Name)))
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, tarReader)
			return
		}
	}
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
