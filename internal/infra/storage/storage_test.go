package storage

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickExtension(t *testing.T) {
	assert.Equal(t, ".png", pickExtension("avatar.PNG", ""))
	assert.Equal(t, ".jpg", pickExtension("", "image/jpeg"))
	assert.Equal(t, ".bin", pickExtension("", ""))
	assert.Equal(t, ".bin", pickExtension("noext", "application/zip"))
	assert.Equal(t, ".svg", pickExtension("", "image/svg+xml"))
	// 扩展名超过 6 字符忽略，回落到 content-type
	assert.Equal(t, ".png", pickExtension("file.toolongext", "image/png"))
}

func TestSaveBytes_WritesFileAndReturnsMeta(t *testing.T) {
	dir := t.TempDir()
	data := []byte("hello world")
	saved, err := SaveBytes(SaveOptions{
		Bytes:       data,
		ContentType: "image/png",
		UploadDir:   dir,
		BaseURL:     "http://localhost:3000",
	})
	require.NoError(t, err)

	assert.Regexp(t, regexp.MustCompile(`^\d+-[0-9a-f]{12}\.png$`), saved.Filename)
	assert.Equal(t, "http://localhost:3000/uploads/"+saved.Filename, saved.URL)
	assert.Equal(t, int64(len(data)), saved.Size)
	assert.Equal(t, "image/png", saved.ContentType)

	content, err := os.ReadFile(filepath.Join(dir, saved.Filename))
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestSaveBytes_DefaultContentType(t *testing.T) {
	dir := t.TempDir()
	saved, err := SaveBytes(SaveOptions{
		Bytes:     []byte("x"),
		UploadDir: dir,
		BaseURL:   "http://localhost:3000",
	})
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", saved.ContentType)
	assert.Equal(t, ".bin", filepath.Ext(saved.Filename))
}
