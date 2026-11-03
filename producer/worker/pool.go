package worker

import (
	"context"
	"log/slog"
	"sync"

	"github.com/zrygan/media-upload-service/producer/network"
)

// MakeWorkers returns a slice of ProducerWorker structs, one for each
// folder. These ProducerWorkers are also assigned a unique ID and
// unique folder assignment. This does not spawn threads and does not
// execute the workers. See RunThreads to see how threads are spawned
// and executed.
func MakeWorkers(folders []string) []ProducerWorker {
	pws := make([]ProducerWorker, len(folders))

	for id, assignmentFolder := range folders {
		pws[id] = ProducerWorker{
			id:         id,
			folderName: assignmentFolder,
		}
	}

	return pws
}

// RunThreads takes in a slice of ProducerWorkers (most likely
// created by MakeWorkers) and runs each one on its own
// goroutine/thread.
//
// This is a BLOCKING function, waits until all goroutines are
// finished.
func RunThreads(ctx context.Context, pws []ProducerWorker, client *network.Client) {
	var wg sync.WaitGroup

	for i := range pws {
		wg.Go(func() {
			if err := pws[i].run(ctx, client); err != nil {
				slog.Error("worker failed", "component", "worker", "worker_id", pws[i].id, "error", err)
			}
		})
	}

	wg.Wait()
}
