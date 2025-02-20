package zipfast

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"database/sql"
	"fmt"
	_ "github.com/glebarez/go-sqlite"
	"io"
	"os"
	"time"
)

type FastZipReader struct {
	db *sql.DB
}

// NewFastZipReader Initialize the database and tables if needed.
func NewFastZipReader(dbPath string) (*FastZipReader, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := initDB(db); err != nil {
		db.Close()
		return nil, err
	}

	return &FastZipReader{db: db}, nil
}

// Close the database connection.
func (zi *FastZipReader) Close() error {
	return zi.db.Close()
}

// Initialize database tables.
func initDB(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS lookup_zip_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		zip_path TEXT UNIQUE NOT NULL,
		size INTEGER NOT NULL,
		modification_time INTEGER NOT NULL,
		indexed_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS lookup_zip_contents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		zip_id INTEGER NOT NULL,
		file_name TEXT NOT NULL,
		offset INTEGER NOT NULL,
		compressed_size INTEGER NOT NULL,
		uncompressed_size INTEGER NOT NULL,
		compression_method INTEGER NOT NULL,
		FOREIGN KEY(zip_id) REFERENCES lookup_zip_files(id),
		UNIQUE(zip_id, file_name)
	);
	`
	_, err := db.Exec(query)
	return err
}

// Indexes a ZIP file, reindexing if it has changed.
func (zi *FastZipReader) indexZip(zipPath string) error {
	fileInfo, err := os.Stat(zipPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	var zipID int
	var existingSize int64
	var existingModTime int64
	row := zi.db.QueryRow("SELECT id, size, modification_time FROM lookup_zip_files WHERE zip_path = ?", zipPath)
	err = row.Scan(&zipID, &existingSize, &existingModTime)
	if err == nil && (existingSize != fileInfo.Size() || existingModTime != fileInfo.ModTime().Unix()) {
		// File changed, reindex
		_, _ = zi.db.Exec("DELETE FROM lookup_zip_contents WHERE zip_id = ?", zipID)
		_, _ = zi.db.Exec("DELETE FROM lookup_zip_files WHERE id = ?", zipID)
	} else if err == nil {
		// File unchanged, skip indexing
		return nil
	}

	return zi.indexZipFile(zipPath, fileInfo)
}

// Internal function to index a ZIP file.
func (zi *FastZipReader) indexZipFile(zipPath string, fileInfo os.FileInfo) error {
	file, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer file.Close()

	zipReader, err := zip.NewReader(file, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("failed to create ZIP reader: %w", err)
	}

	tx, err := zi.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO lookup_zip_files (zip_path, size, modification_time, indexed_at) VALUES (?, ?, ?, ?)",
		zipPath, fileInfo.Size(), fileInfo.ModTime().Unix(), time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to insert ZIP file metadata: %w", err)
	}

	zipID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	stmt, err := tx.Prepare("INSERT INTO lookup_zip_contents (zip_id, file_name, offset, compressed_size, uncompressed_size, compression_method) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, f := range zipReader.File {
		offset, err := f.DataOffset()
		if err != nil {
			return fmt.Errorf("failed to get data offset for %s: %w", f.Name, err)
		}

		_, err = stmt.Exec(zipID, f.Name, offset, f.CompressedSize64, f.UncompressedSize64, f.Method)
		if err != nil {
			return fmt.Errorf("failed to insert record for %s: %w", f.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// StreamFile Streams a file from the ZIP archive. The archive gets indexed automatically.
func (zi *FastZipReader) StreamFile(zipPath, filename string, writer io.Writer) error {
	var zipID int
	var row *sql.Row
	row = zi.db.QueryRow("SELECT id FROM lookup_zip_files WHERE zip_path = ?", zipPath)
	if err := row.Scan(&zipID); err != nil {
		err = zi.indexZip(zipPath)
		if err != nil {
			return err
		}
		row = zi.db.QueryRow("SELECT id FROM lookup_zip_files WHERE zip_path = ?", zipPath)
		if err := row.Scan(&zipID); err != nil {
			return fmt.Errorf("database error for file %s", filename)
		}
	}

	var metadata struct {
		Offset            int64
		CompressedSize    uint64
		UncompressedSize  uint64
		CompressionMethod uint16
	}

	err := zi.db.QueryRow("SELECT offset, compressed_size, uncompressed_size, compression_method FROM lookup_zip_contents WHERE zip_id = ? AND file_name = ?", zipID, filename).Scan(&metadata.Offset, &metadata.CompressedSize, &metadata.UncompressedSize, &metadata.CompressionMethod)
	if err != nil {
		return fmt.Errorf("file %s not found in index: %w", filename, err)
	}

	file, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer file.Close()

	compressedData := make([]byte, metadata.CompressedSize)
	_, err = file.Seek(metadata.Offset, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to file offset: %w", err)
	}

	_, err = io.ReadFull(file, compressedData)
	if err != nil {
		return fmt.Errorf("failed to read compressed data: %w", err)
	}

	if metadata.CompressionMethod == zip.Store {
		_, err = writer.Write(compressedData)
		return err
	} else if metadata.CompressionMethod == zip.Deflate {
		r := flate.NewReader(bytes.NewReader(compressedData))
		defer r.Close()
		_, err = io.Copy(writer, r)
		return err
	}

	return fmt.Errorf("unsupported compression method: %d", metadata.CompressionMethod)
}
