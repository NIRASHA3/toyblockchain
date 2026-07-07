package blockchain

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MiningStats describes the work performed while mining a block.
type MiningStats struct {
	Nonce    uint64
	Attempts uint64
	Duration time.Duration
	Workers  int
}

// mineWorker performs nonce search for a single worker.
func mineWorker(ctx context.Context, candidate Block, difficulty int, workerID int, workers int, resultCh chan Block, attempts *uint64) {
	block := candidate
	step := uint64(workers)
	for nonce := uint64(workerID); ; nonce += step {
		select {
		case <-ctx.Done():
			return
		default:
		}

		block.Nonce = nonce
		hash := block.ComputeHash()
		atomic.AddUint64(attempts, 1)
		if MeetsDifficulty(hash, difficulty) {
			block.Hash = hash
			select {
			case resultCh <- block:
			default:
			}
			return
		}

		if nonce > ^uint64(0)-step {
			return
		}
	}
}

// Mine searches for a nonce that makes candidate.Hash satisfy difficulty.
// It splits nonce search across goroutines, uses context cancellation for timeouts,
// and returns the first valid nonce found.
func Mine(ctx context.Context, candidate Block, difficulty int, workers int) (Block, MiningStats, error) {
	if difficulty < 0 || difficulty > MaxDifficulty {
		return Block{}, MiningStats{}, fmt.Errorf("%w: got %d, supported range is 0..%d", ErrInvalidDifficulty, difficulty, MaxDifficulty)
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()
	resultCh := make(chan Block, 1)
	var attempts uint64
	var wg sync.WaitGroup

	for workerID := 0; workerID < workers; workerID++ {
		workerID := workerID
		wg.Add(1)
		go func() {
			defer wg.Done()
			mineWorker(ctx, candidate, difficulty, workerID, workers, resultCh, &attempts)
		}()
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case block := <-resultCh:
		<-doneCh
		return block, MiningStats{Nonce: block.Nonce, Attempts: atomic.LoadUint64(&attempts), Duration: time.Since(start), Workers: workers}, nil
	case <-ctx.Done():
		<-doneCh
		return Block{}, MiningStats{Attempts: atomic.LoadUint64(&attempts), Duration: time.Since(start), Workers: workers}, fmt.Errorf("mining cancelled: %w", ctx.Err())
	}
}
