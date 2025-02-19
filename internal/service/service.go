package service

import (
	"cmpserve/internal/readers/zip"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	rootServiceDir  string
	cacheServiceDir string
	zipReader       zip.FastZipReader
	createIndexes   bool
}

func NewService(rootServiceDir, cacheServiceDir string, createIndexes bool) (*Service, error) {
	rootServiceDir = filepath.Clean(rootServiceDir)
	cacheServiceDir = filepath.Clean(cacheServiceDir)
	if stat, err := os.Stat(rootServiceDir); err != nil || !stat.IsDir() {
		return nil, errors.New("invalid service directory")
	}
	if stat, err := os.Stat(cacheServiceDir); err != nil || !stat.IsDir() {
		return nil, errors.New("invalid cache directory")
	}
	zipReader, err := zip.NewFastZipReader(cacheServiceDir + "/zip_reader_cache.db")
	if err != nil {
		return nil, err
	}
	return &Service{rootServiceDir: rootServiceDir, cacheServiceDir: cacheServiceDir, zipReader: *zipReader, createIndexes: createIndexes}, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(urlPath, "/")

	currentPath := s.rootServiceDir
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

		archiveCandidate := currentPath + ".zip"
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

	if archivePath == "" {
		http.NotFound(w, r)
		return
	}

	if remainingPath == "" {
		remainingPath = "index.html"
	}

	err := s.zipReader.StreamFile(archivePath, remainingPath, w)
	if err != nil {
		http.NotFound(w, r)
		return
	}
}
