package worker

import (
	"log/slog"
	"sync"

	"github.com/zrygan/media-upload-service/consumer/queue"
)

// MakeWorkers returns a slice of threadCount ConsumerWorker structs.
// Each worker gets a unique ID, and all of them share the same queue
// q and save into the same mediaFolder. This does not spawn threads
// and does not execute the workers. See RunThreads to see how threads
// are spawned and executed.
func MakeWorkers(threadCount int, q *queue.Queue, mediaFolder string) []ConsumerWorker {
	cws := make([]ConsumerWorker, threadCount)

	for id := 0; id < threadCount; id++ {
		cws[id] = ConsumerWorker{
			id:    id,
			q:     q,
			media: mediaFolder,
		}
	}

	return cws
}

// RunThreads takes in a slice of ConsumerWorkers (most likely
// created by MakeWorkers) and runs each one on its own
// goroutine/thread.
//
// This is a BLOCKING function, waits until all goroutines are
// finished. The workers finish once the queue is closed and drained.
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
