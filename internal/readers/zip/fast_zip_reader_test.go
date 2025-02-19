package zip

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Helper to create a temporary ZIP file with given files.
func createTestZip(t *testing.T, zipPath string, files map[string]string) {
	t.Helper()
	outFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create test zip file: %v", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	for name, content := range files {
		w, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("Failed to add file to zip: %v", err)
		}
		_, err = w.Write([]byte(content))
		if err != nil {
			t.Fatalf("Failed to write content to zip file: %v", err)
		}
	}
	zipWriter.Close()
}

// Setup function to create an in-memory SQLite database.
func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	if err := initDB(db); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	return db
}

func TestIndexZipFile(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	zipPath := "test.zip"
	defer os.Remove(zipPath)

	files := map[string]string{
		"file1.txt": "Hello, World!",
		"file2.txt": "Test file content",
	}
	createTestZip(t, zipPath, files)

	err := indexZipFile(db, zipPath)
	if err != nil {
		t.Fatalf("Indexing failed: %v", err)
	}

	// Check if files exist in the database
	for filename := range files {
		var count int
		query := `SELECT COUNT(*) FROM lookup_zip WHERE zip_filename = ? AND file_name = ?`
		err := db.QueryRow(query, zipPath, filename).Scan(&count)
		if err != nil {
			t.Fatalf("Database query failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("File %s not indexed properly", filename)
		}
	}
}

func TestGetFileMetadata(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	zipPath := "test.zip"
	defer os.Remove(zipPath)

	files := map[string]string{
		"file1.txt": "Hello, World!",
	}
	createTestZip(t, zipPath, files)

	err := indexZipFile(db, zipPath)
	if err != nil {
		t.Fatalf("Indexing failed: %v", err)
	}

	_, err = getFileMetadata(db, zipPath, "file1.txt")
	if err != nil {
		t.Fatalf("Metadata retrieval failed: %v", err)
	}

	// Non-existent file should return error
	_, err = getFileMetadata(db, zipPath, "nonexistent.txt")
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}
}

func TestExtractFile(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	zipPath := "test.zip"
	defer os.Remove(zipPath)

	files := map[string]string{
		"file1.txt": "Hello, World!",
	}
	createTestZip(t, zipPath, files)

	err := indexZipFile(db, zipPath)
	if err != nil {
		t.Fatalf("Indexing failed: %v", err)
	}

	var buffer bytes.Buffer
	err = StreamFile(db, zipPath, "file1.txt", &buffer)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	data := buffer.String()

	expected := "Hello, World!"
	if string(data) != expected {
		t.Fatalf("Extracted content mismatch. Got: %s, Expected: %s", string(data), expected)
	}
}

func TestExtractNonExistentFile(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	zipPath := "test.zip"
	defer os.Remove(zipPath)

	files := map[string]string{
		"file1.txt": "Hello, World!",
	}
	createTestZip(t, zipPath, files)

	err := indexZipFile(db, zipPath)
	if err != nil {
		t.Fatalf("Indexing failed: %v", err)
	}

	var buffer bytes.Buffer
	err = StreamFile(db, zipPath, "file1.txt", &buffer)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
}

func TestReindexingDoesNotDuplicateEntries(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	zipPath := "test.zip"
	defer os.Remove(zipPath)

	files := map[string]string{
		"file1.txt": "Hello, World!",
	}
	createTestZip(t, zipPath, files)

	err := indexZipFile(db, zipPath)
	if err != nil {
		t.Fatalf("Indexing failed: %v", err)
	}

	// Re-indexing should not create duplicates
	err = indexZipFile(db, zipPath)
	if err != nil {
		t.Fatalf("Re-indexing failed: %v", err)
	}

	var count int
	query := `SELECT COUNT(*) FROM lookup_zip WHERE zip_filename = ? AND file_name = ?`
	err = db.QueryRow(query, zipPath, "file1.txt").Scan(&count)
	if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("File was duplicated in index")
	}
}

func TestIndexCorruptZip(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	zipPath := "corrupt.zip"
	defer os.Remove(zipPath)

	// Create a corrupt ZIP file
	_ = os.WriteFile(zipPath, []byte("not a zip file"), 0644)

	err := indexZipFile(db, zipPath)
	if err == nil {
		t.Fatal("Expected error when indexing corrupt ZIP, got nil")
	}
}

func TestExtractFileWithCorruptIndex(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	zipPath := "test.zip"
	defer os.Remove(zipPath)

	files := map[string]string{
		"file1.txt": "Hello, World!",
	}
	createTestZip(t, zipPath, files)

	err := indexZipFile(db, zipPath)
	if err != nil {
		t.Fatalf("Indexing failed: %v", err)
	}

	// Manually corrupt the database entry
	_, err = db.Exec(`UPDATE lookup_zip SET offset = 999999 WHERE file_name = ?`, "file1.txt")
	if err != nil {
		t.Fatalf("Failed to corrupt index: %v", err)
	}

	var buffer bytes.Buffer
	err = StreamFile(db, zipPath, "file1.txt", &buffer)
	if err == nil {
		t.Fatal("Expected error due to corrupt index, but got nil")
	}
}
