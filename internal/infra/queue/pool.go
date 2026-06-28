// Package queue 提供有界任务队列 + 固定数量 worker，支持优雅关闭。
// 它是 infra 层的并发基础设施，向上对调用者暴露 Submit / Shutdown 语义，
// 向下注入可替换的 SaveFunc（默认接 storage.SaveBytes）。
package queue

import (
	"context"
	"errors"
	"sync"

	"d2c-manager/internal/infra/storage"
)

// ErrShuttingDown 在池已经开始关闭后提交任务时返回。
var ErrShuttingDown = errors.New("queue: shutting down")

// SaveFunc 是被注入的持久化实现（默认 storage.SaveBytes）。
type SaveFunc func(storage.SaveOptions) (storage.SavedFile, error)

// PersistResult 通过 PersistJob.Result 回传给提交者。
type PersistResult struct {
	Saved storage.SavedFile
	Err   error
}

// PersistJob 是一次写盘任务；Result 必须是带缓冲(1)的 channel，避免 worker 阻塞。
type PersistJob struct {
	Options storage.SaveOptions
	Result  chan PersistResult
}

// Pool 是有界任务队列 + 固定数量 worker，支持优雅关闭（排空后退出）。
type Pool struct {
	jobs      chan *PersistJob
	quit      chan struct{}
	workers   int
	save      SaveFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
}

func NewPool(workers, queueSize int, save SaveFunc) *Pool {
	if workers <= 0 {
		workers = 1
	}
	if queueSize < 0 {
		queueSize = 0
	}
	return &Pool{
		jobs:    make(chan *PersistJob, queueSize),
		quit:    make(chan struct{}),
		workers: workers,
		save:    save,
	}
}

func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case job := <-p.jobs:
			p.process(job)
		case <-p.quit:
			// 关闭信号：排空缓冲区里剩余的任务后退出。
			for {
				select {
				case job := <-p.jobs:
					p.process(job)
				default:
					return
				}
			}
		}
	}
}

func (p *Pool) process(job *PersistJob) {
	saved, err := p.save(job.Options)
	job.Result <- PersistResult{Saved: saved, Err: err}
}

// Submit 把任务入队。池关闭后返回 ErrShuttingDown；ctx 取消则返回 ctx.Err()。
// 注意：从不 close(p.jobs)，因此不存在向已关闭 channel 发送的 panic。
func (p *Pool) Submit(ctx context.Context, job *PersistJob) error {
	select {
	case <-p.quit:
		return ErrShuttingDown
	default:
	}
	select {
	case p.jobs <- job:
		return nil
	case <-p.quit:
		return ErrShuttingDown
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown 停止接收新任务，等待 worker 排空并退出；ctx 超时则提前返回 ctx.Err()。
func (p *Pool) Shutdown(ctx context.Context) error {
	p.closeOnce.Do(func() { close(p.quit) })

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}