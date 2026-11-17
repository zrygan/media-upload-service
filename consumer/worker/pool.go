package worker

import (
	"context"
	"log/slog"
	"sync"

	"github.com/zrygan/media-upload-service/consumer/dedup"
	"github.com/zrygan/media-upload-service/consumer/inflight"
	"github.com/zrygan/media-upload-service/consumer/queue"
)

// MakeWorkers returns threadCount workers that share q, idx,
// mediaFolder, the name mutex and h. It does not start them (see
// RunThreads).
func MakeWorkers(threadCount int, q *queue.Queue, idx *dedup.Index, mediaFolder string, mut *sync.Mutex, h *inflight.Hashbrowns) []ConsumerWorker {
	cws := make([]ConsumerWorker, threadCount)

	for id := 0; id < threadCount; id++ {
		cws[id] = ConsumerWorker{
			id:    id,
			q:     q,
			idx:   idx,
			media: mediaFolder,
			mut:   mut,
			h:     h,
		}
	}

	return cws
}

// RunThreads runs each worker on its own goroutine and blocks until all
// of them return, which happens once the queue is closed and empty.
// When ctx ends, running compressions stop and the originals are saved.
func RunThreads(ctx context.Context, cws []ConsumerWorker) {
	var wg sync.WaitGroup

	for i := range cws {
		wg.Go(func() {
			if err := cws[i].run(ctx); err != nil {
				slog.Error("worker failed", "component", "worker", "worker_id", cws[i].id, "error", err)
			}
		})
	}

	wg.Wait()
}
