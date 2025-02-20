package zipfast

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"archive/zip"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestZipFile(zipPath string, files map[string]string) error {
	file, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	for name, content := range files {
		w, err := zipWriter.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		if err != nil {
			return err
		}
	}
	return nil
}

func TestFastZipReader(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	zipPath := filepath.Join(tempDir, "test.zip")

	files := map[string]string{
		"file1.txt": "Hello, World!",
		"file2.txt": "Another file content",
	}
	require.NoError(t, createTestZipFile(zipPath, files))

	reader, err := NewFastZipReader(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })

	// Ensure database tables were created
	var count int
	err = reader.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='lookup_zip_files'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	err = reader.indexZip(zipPath)
	require.NoError(t, err)

	// Check if ZIP metadata is stored
	var zipID int
	err = reader.db.QueryRow("SELECT id FROM lookup_zip_files WHERE zip_path = ?", zipPath).Scan(&zipID)
	require.NoError(t, err)

	// Check if file entries exist in the database
	var fileCount int
	err = reader.db.QueryRow("SELECT count(*) FROM lookup_zip_contents WHERE zip_id = ?", zipID).Scan(&fileCount)
	require.NoError(t, err)
	assert.Equal(t, len(files), fileCount)

	// Test streaming a file
	var output bytes.Buffer
	err = reader.StreamFile(zipPath, "file1.txt", &output)
	require.NoError(t, err)
	assert.Equal(t, files["file1.txt"], output.String())
}

func TestReindexingZip(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	zipPath := filepath.Join(tempDir, "test.zip")

	files := map[string]string{
		"file1.txt": "Original content",
	}
	require.NoError(t, createTestZipFile(zipPath, files))

	reader, err := NewFastZipReader(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })

	require.NoError(t, reader.indexZip(zipPath))

	// Modify ZIP file
	time.Sleep(time.Second) // Ensure modification timestamp changes
	files["file1.txt"] = "Updated content"
	require.NoError(t, createTestZipFile(zipPath, files))

	require.NoError(t, reader.indexZip(zipPath))

	var output bytes.Buffer
	require.NoError(t, reader.StreamFile(zipPath, "file1.txt", &output))
	assert.Equal(t, files["file1.txt"], output.String())
}
