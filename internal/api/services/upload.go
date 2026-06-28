package services

import (
	"context"
	"errors"

	"d2c-manager/internal/infra/queue"
	"d2c-manager/internal/infra/storage"
)

// ErrUnavailable 标记"请求未能进入处理"这一类失败：池正在关闭、入队失败或上下文已取消。
// handlers 层用 errors.Is 命中它回 503；其余（持久化本身失败）回 500。
// 包装原因可经 errors.Unwrap 取回，供 handler 写入 message。
var ErrUnavailable = errors.New("server unavailable")

// unavailable 把底层原因包成 ErrUnavailable 家族的错误：
// errors.Is(err, ErrUnavailable) 命中，errors.Unwrap(err) 取回原因。
func unavailable(cause error) error {
	return &unavailableError{cause: cause}
}

type unavailableError struct{ cause error }

func (e *unavailableError) Error() string { return ErrUnavailable.Error() + ": " + e.cause.Error() }
func (e *unavailableError) Unwrap() error { return e.cause }
func (e *unavailableError) Is(target error) bool {
	return target == ErrUnavailable
}

// UploadService 把上传字节交给 worker 池异步落盘，并同步等待结果。
type UploadService struct {
	pool *queue.Pool
}

// NewUploadService 绑定底层 worker 池。
func NewUploadService(pool *queue.Pool) *UploadService {
	return &UploadService{pool: pool}
}

// Save 提交一次写盘任务并等待结果。
// 入队失败或 ctx 取消 → ErrUnavailable（可 Unwrap 出原因）；持久化失败 → 原始错误。
func (s *UploadService) Save(ctx context.Context, opts storage.SaveOptions) (storage.SavedFile, error) {
	job := &queue.PersistJob{
		Options: opts,
		Result:  make(chan queue.PersistResult, 1),
	}

	if err := s.pool.Submit(ctx, job); err != nil {
		return storage.SavedFile{}, unavailable(err)
	}

	select {
	case res := <-job.Result:
		if res.Err != nil {
			return storage.SavedFile{}, res.Err
		}
		return res.Saved, nil
	case <-ctx.Done():
		return storage.SavedFile{}, unavailable(ctx.Err())
	}
}
