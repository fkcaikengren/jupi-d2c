package services

import (
	"context"
	"errors"
	"testing"

	"jupi-d2c/internal/infra/queue"
	"jupi-d2c/internal/infra/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPool 起一个注入了自定义 SaveFunc 的池，测试结束时关闭。
func newPool(t *testing.T, save queue.SaveFunc) *queue.Pool {
	t.Helper()
	p := queue.NewPool(2, 8, save)
	p.Start()
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	return p
}

// 成功路径：返回池处理出的 SavedFile。
func TestUploadService_Save_OK(t *testing.T) {
	want := storage.SavedFile{Size: 7, ContentType: "image/png", URL: "http://x/uploads/a.png"}
	p := newPool(t, func(storage.SaveOptions) (storage.SavedFile, error) { return want, nil })

	got, err := NewUploadService(p).Save(context.Background(), storage.SaveOptions{Bytes: []byte("PNGDATA")})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// 持久化失败：返回原始错误，且不属于 ErrUnavailable 家族（handler 回 500）。
func TestUploadService_Save_ProcessingError(t *testing.T) {
	boom := errors.New("disk full")
	p := newPool(t, func(storage.SaveOptions) (storage.SavedFile, error) { return storage.SavedFile{}, boom })

	_, err := NewUploadService(p).Save(context.Background(), storage.SaveOptions{Bytes: []byte("x")})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUnavailable))
	assert.ErrorIs(t, err, boom)
}

// 入队失败（池已关闭）：归类为 ErrUnavailable（handler 回 503），可 Unwrap 出原因。
func TestUploadService_Save_Unavailable(t *testing.T) {
	p := queue.NewPool(1, 1, func(storage.SaveOptions) (storage.SavedFile, error) {
		return storage.SavedFile{}, nil
	})
	p.Start()
	require.NoError(t, p.Shutdown(context.Background()))

	_, err := NewUploadService(p).Save(context.Background(), storage.SaveOptions{Bytes: []byte("x")})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.ErrorIs(t, err, queue.ErrShuttingDown)
}
