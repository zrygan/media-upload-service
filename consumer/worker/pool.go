package worker

import (
	"log/slog"
	"sync"

	"github.com/zrygan/media-upload-service/consumer/inflight"
	"github.com/zrygan/media-upload-service/consumer/queue"
)

// MakeWorkers returns threadCount workers that share q, mediaFolder,
// the name mutex and h. It does not start them (see RunThreads).
func MakeWorkers(threadCount int, q *queue.Queue, mediaFolder string, mut *sync.Mutex, h *inflight.Hashbrowns) []ConsumerWorker {
	cws := make([]ConsumerWorker, threadCount)

	for id := 0; id < threadCount; id++ {
		cws[id] = ConsumerWorker{
			id:    id,
			q:     q,
			media: mediaFolder,
			mut:   mut,
			h:     h,
		}
	}

	return cws
}

// RunThreads runs each worker on its own goroutine and blocks until all
// of them return, which happens once the queue is closed and empty.
func RunThreads(cws []ConsumerWorker) {
	var wg sync.WaitGroup

	for i := range cws {
		wg.Go(func() {
			if err := cws[i].run(); err != nil {
				slog.Error("worker failed", "component", "worker", "worker_id", cws[i].id, "error", err)
			}
		})
	}

	wg.Wait()
}
