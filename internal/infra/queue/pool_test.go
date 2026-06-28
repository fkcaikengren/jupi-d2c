package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"jupi-d2c/internal/infra/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPool_ProcessesJob(t *testing.T) {
	save := func(o storage.SaveOptions) (storage.SavedFile, error) {
		return storage.SavedFile{Filename: "f", Size: int64(len(o.Bytes))}, nil
	}
	p := NewPool(2, 8, save)
	p.Start()
	defer p.Shutdown(context.Background())

	job := &PersistJob{
		Options: storage.SaveOptions{Bytes: []byte("hi")},
		Result:  make(chan PersistResult, 1),
	}
	require.NoError(t, p.Submit(context.Background(), job))

	select {
	case res := <-job.Result:
		require.NoError(t, res.Err)
		assert.Equal(t, int64(2), res.Saved.Size)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for job result")
	}
}

func TestPool_PropagatesError(t *testing.T) {
	boom := errors.New("boom")
	save := func(o storage.SaveOptions) (storage.SavedFile, error) {
		return storage.SavedFile{}, boom
	}
	p := NewPool(1, 1, save)
	p.Start()
	defer p.Shutdown(context.Background())

	job := &PersistJob{Options: storage.SaveOptions{}, Result: make(chan PersistResult, 1)}
	require.NoError(t, p.Submit(context.Background(), job))
	res := <-job.Result
	assert.ErrorIs(t, res.Err, boom)
}

func TestPool_SubmitAfterShutdown(t *testing.T) {
	save := func(o storage.SaveOptions) (storage.SavedFile, error) {
		return storage.SavedFile{}, nil
	}
	p := NewPool(1, 1, save)
	p.Start()
	require.NoError(t, p.Shutdown(context.Background()))

	job := &PersistJob{Options: storage.SaveOptions{Bytes: []byte("x")}, Result: make(chan PersistResult, 1)}
	err := p.Submit(context.Background(), job)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrShuttingDown))
}
