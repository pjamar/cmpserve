package zip

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Initialize the SQLite table if it doesn't exist.
func initDB(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS lookup_zip (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		zip_filename TEXT NOT NULL,
		file_name TEXT NOT NULL,
		offset INTEGER NOT NULL,
		compressed_size INTEGER NOT NULL,
		uncompressed_size INTEGER NOT NULL,
		compression_method INTEGER NOT NULL,
		indexed_at DATETIME NOT NULL,
		UNIQUE(zip_filename, file_name)
	);
	`
	_, err := db.Exec(query)
	return err
}

// Indexes a ZIP file in the SQLite database.
func indexZipFile(db *sql.DB, zipFilename string) error {
	// Open the ZIP file
	file, err := os.Open(zipFilename)
	if err != nil {
		return fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	zipReader, err := zip.NewReader(file, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("failed to create ZIP reader: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Will be ignored if tx.Commit() is called

	stmt, err := tx.Prepare(`
	INSERT OR IGNORE INTO lookup_zip 
	(zip_filename, file_name, offset, compressed_size, uncompressed_size, compression_method, indexed_at) 
	VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	indexedAt := time.Now().Format(time.RFC3339)

	for _, f := range zipReader.File {
		if f.Method != zip.Store && f.Method != zip.Deflate {
			continue // Skip unsupported compression methods
		}

		offset, err := f.DataOffset()
		if err != nil {
			return fmt.Errorf("failed to get data offset for %s: %w", f.Name, err)
		}

		_, err = stmt.Exec(
			zipFilename,
			f.Name,
			offset,
			f.CompressedSize64,
			f.UncompressedSize64,
			f.Method,
			indexedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert record for %s: %w", f.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// FileMetadata holds information about a file in the ZIP archive
type FileMetadata struct {
	Offset            int64
	CompressedSize    uint64
	UncompressedSize  uint64
	CompressionMethod uint16
}

// Check if the file exists in the index and retrieve its metadata.
func getFileMetadata(db *sql.DB, zipFilename, filename string) (*FileMetadata, error) {
	var metadata FileMetadata

	query := `
		SELECT offset, compressed_size, uncompressed_size, compression_method 
		FROM lookup_zip 
		WHERE zip_filename = ? AND file_name = ?`

	err := db.QueryRow(query, zipFilename, filename).Scan(
		&metadata.Offset,
		&metadata.CompressedSize,
		&metadata.UncompressedSize,
		&metadata.CompressionMethod,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("file %s not found in index", filename)
	} else if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	return &metadata, nil
}

// Extract file contents using the pre-indexed metadata.
func extractFile(db *sql.DB, zipFilename, filename string) ([]byte, error) {
	metadata, err := getFileMetadata(db, zipFilename, filename)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(zipFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer file.Close()

	// Read the compressed data
	compressedData := make([]byte, metadata.CompressedSize)
	_, err = file.Seek(metadata.Offset, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to file offset: %w", err)
	}

	_, err = io.ReadFull(file, compressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	// For stored (uncompressed) files, return the data as-is
	if metadata.CompressionMethod == zip.Store {
		return compressedData, nil
	}

	// For deflated files, decompress the data
	if metadata.CompressionMethod == zip.Deflate {
		// Create a flate reader from the compressed data
		r := flate.NewReader(bytes.NewReader(compressedData))
		defer r.Close()

		// Read all the uncompressed data
		uncompressedData, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress data: %w", err)
		}

		return uncompressedData, nil
	}

	return nil, fmt.Errorf("unsupported compression method: %d", metadata.CompressionMethod)
}

//func main() {
//	db, err := sql.Open("sqlite3", "zip_index.db")
//	if err != nil {
//		fmt.Fprintf(os.Stderr, "Database connection error: %v\n", err)
//		os.Exit(1)
//	}
//	defer db.Close()
//
//	if err := initDB(db); err != nil {
//		fmt.Fprintf(os.Stderr, "DB initialization error: %v\n", err)
//		os.Exit(1)
//	}
//
//	if len(os.Args) < 3 {
//		fmt.Println("Usage: program <zip_file> <file_to_extract>")
//		os.Exit(1)
//	}
//
//	zipFilename := os.Args[1]
//	filenameToRetrieve := os.Args[2]
//
//	// Index the ZIP file if it hasn't been indexed
//	if err := indexZipFile(db, zipFilename); err != nil {
//		fmt.Fprintf(os.Stderr, "Error indexing ZIP file: %v\n", err)
//		os.Exit(1)
//	}
//
//	// Retrieve and print file contents
//	data, err := extractFile(db, zipFilename, filenameToRetrieve)
//	if err != nil {
//		fmt.Fprintf(os.Stderr, "Error extracting file: %v\n", err)
//		os.Exit(1)
//	}
//
//	fmt.Printf("Successfully extracted %s (%d bytes)\n", filenameToRetrieve, len(data))
//	os.Stdout.Write(data)
//}
