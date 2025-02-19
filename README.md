# Comprehensive Documentation for cmpserve

## Overview
`cmpserve` is a lightweight HTTP server designed to serve files and directories, with enhanced support for ZIP archives using an indexed caching system for fast access. The project provides an efficient way to expose files over HTTP while optionally displaying directory indexes and handling hidden files.

## Features
- Serves files and directories over HTTP.
- Supports ZIP file indexing for fast access.
- Allows optional directory listing.
- Configurable options for hidden files and indexing.
- Caches ZIP file metadata using SQLite.

---

## Project Structure
```
cmpserve/
│── main.go               # Entry point of the application
│── internal/
│   ├── service/
│   │   ├── service.go    # HTTP handler and service initialization
│   ├── readers/
│   │   ├── zipfast/
│   │   │   ├── fast_zip_reader.go  # Optimized ZIP file reader with SQLite index
```

---

## Configuration and Usage

### Command-Line Flags
`cmpserve` accepts several optional command-line flags:

| Flag                 | Default Value | Description |
|----------------------|---------------|-------------|
| `-dir`              | `.`           | Root directory to serve |
| `-cache-dir`        | `.`           | Directory for cache storage |
| `-addr`             | `0.0.0.0`     | Bind address for the server |
| `-port`             | `8080`        | Port to listen on |
| `-indexes`          | `false`       | Whether to display directory indexes |
| `-show-hidden-files`| `false`       | Whether to serve hidden files |

### Running the Server
Run the server with:
```sh
./cmpserve -dir=/path/to/serve -cache-dir=/path/to/cache -port=9090
```

---

## Implementation Details

### `main.go`
Handles command-line arguments, initializes the service, and starts an HTTP server.

### `service.go`
- Implements `ServeHTTP`, handling file requests, directory indexing, and ZIP file streaming.
- Routes requests based on path structure.
- Supports automatic directory listing when enabled.

### `fast_zip_reader.go`
- Uses SQLite to store metadata of ZIP archives.
- Caches ZIP file entries to enable quick retrieval.
- Provides `StreamFile` for extracting and serving specific files from ZIP archives.
- Supports `Deflate` and `Store` compression methods.

---

## API Behavior

### Serving Files
- Directories are served with index listings if `-indexes` is enabled.
- ZIP files are dynamically indexed and extracted on request.

### Streaming ZIP Files
If a requested path points to a file inside a ZIP archive, the server:
1. Checks if the ZIP file is indexed.
2. If not, indexes it and caches the metadata.
3. Streams the requested file from the archive.

### Handling Directories
- If a directory is requested, it displays an index if enabled.
- If `show-hidden-files` is disabled, hidden files are omitted.

---

## Error Handling
- Logs initialization failures.
- Returns `404 Not Found` for missing files or inaccessible paths.
- Returns `500 Internal Server Error` for database or indexing issues.
