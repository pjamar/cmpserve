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
	rootServiceDir    string
	cacheServiceDir   string
	zipReader         zip.FastZipReader
	createIndexes     bool
	exposeHiddenFiles bool
}

func NewService(rootServiceDir, cacheServiceDir string, createIndexes bool, exposeHiddenFiles bool) (*Service, error) {
	rootServiceDir = filepath.Clean(rootServiceDir)
	cacheServiceDir = filepath.Clean(cacheServiceDir)
	if stat, err := os.Stat(rootServiceDir); err != nil || !stat.IsDir() {
		return nil, errors.New("invalid service directory")
	}
	if stat, err := os.Stat(cacheServiceDir); err != nil || !stat.IsDir() {
		return nil, errors.New("invalid cache directory")
	}
	zipReader, err := zip.NewFastZipReader(cacheServiceDir + "/.zip_reader_cache.db")
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

		if !s.exposeHiddenFiles && strings.HasPrefix(part, ".") {
			http.NotFound(w, r)
			return
		}

		if stat, err := os.Stat(currentPath); err == nil {
			if stat.IsDir() {
				// If it's a directory and createIndexes is enabled, list the directory contents
				if i == len(parts)-1 && s.createIndexes {
					s.listDirectory(w, currentPath, urlPath)
					return
				}
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
		if s.createIndexes {
			s.listDirectory(w, currentPath, urlPath)
			return
		}
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

func (s *Service) listDirectory(w http.ResponseWriter, dirPath, urlPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<html><body><h1>Index of " + urlPath + "</h1><ul>"))

	for _, entry := range entries {
		name := entry.Name()
		if !s.exposeHiddenFiles && strings.HasPrefix(name, ".") {
			continue
		}

		linkName := filepath.Join(urlPath, name)
		var extraLink string

		if entry.IsDir() {
			name += "/"
			linkName = name
		} else if strings.HasSuffix(name, ".zip") {
			nameWithoutExt := strings.TrimSuffix(name, ".zip") + "/"
			linkName = nameWithoutExt
			extraLink = " (<a href=\"" + name + "\">download</a>)"
		}

		w.Write([]byte("<li><a href=\"" + linkName + "\">" + name + "</a>" + extraLink + "</li>"))
	}

	w.Write([]byte("</ul></body></html>"))
}
