# cmpserve

An HTTP server to display zip files as folders, making it easy to serve compact static archives.

## Overview
`cmpserve` is a lightweight HTTP server designed to serve ZIP files as they were directories. It also would serve files and directories. ZIP archives get indexed for quicker random access.

Optionally `cmpserve` can display directory indexes and expose hidden files.

## Features
- Serves files and directories over HTTP.
- Supports ZIP file indexing for fast access.
- Allows optional directory listing.
- Configurable options for hidden files and indexing.
- Caches ZIP file metadata using SQLite.
- Supports configuration via environment variables.

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

### Environment Variables
As an alternative to command-line flags, `cmpserve` allows configuration using environment variables. Command-line flags take precedence over environment variables.

| Environment Variable            | Default Value | Description |
|---------------------------------|---------------|-------------|
| `CMPSERVE_DIR`                 | `.`           | Root directory to serve |
| `CMPSERVE_CACHE_DIR`           | `.`           | Directory for cache storage |
| `CMPSERVE_ADDR`                | `0.0.0.0`     | Bind address for the server |
| `CMPSERVE_PORT`                | `8080`        | Port to listen on |
| `CMPSERVE_INDEXES`             | `false`       | Whether to display directory indexes (set to `true` to enable) |
| `CMPSERVE_SHOW_HIDDEN_FILES`   | `false`       | Whether to serve hidden files (set to `true` to enable) |

### Running the Server
Run the server with:
```sh
./cmpserve -dir=/path/to/serve -cache-dir=/path/to/cache -port=9090
```

Or using environment variables:
```sh
export CMPSERVE_DIR="/path/to/serve"
export CMPSERVE_CACHE_DIR="/path/to/cache"
export CMPSERVE_PORT="9090"
./cmpserve
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

