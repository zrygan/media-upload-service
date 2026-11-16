package worker

import (
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/zrygan/media-upload-service/consumer/dedup"
	"github.com/zrygan/media-upload-service/consumer/queue"
)

// ConsumerWorker moves finished uploads from the staging folder into
// the media folder. Each ConsumerWorker runs on its own
// goroutine/thread, see RunThreads.
type ConsumerWorker struct {
	id    int
	q     *queue.Queue
	idx   *dedup.Index
	media string
}

// run takes jobs off the queue until the queue is closed and empty,
// and moves each staged file into the media folder. A failed move is
// logged, the staged file and its duplicate index entry are removed,
// and the worker continues with the next job. This currently always
// returns nil.
//
// All workers receive from the same channel, and each job is received
// by only one worker, so no lock is needed.
//
// NOTE: os.Rename moves the whole file at once, so the GUI never sees
// a half-written video. This only works because the staging and media
// folders are on the same filesystem.
func (c *ConsumerWorker) run() error {
	js := c.q.Jobs()
	for {

		j, ok := <-js

		if !ok {
			break
		}
		// the job left the queue, so its slot is free again
		c.q.UnReserve()

		// saved as <hash>_<name>, filepath.Base keeps it inside the
		// media folder
		id := hex.EncodeToString(j.Hash)
		newName := id + "_" + filepath.Base(j.FileName)
		newPath := filepath.Join(c.media, newName)

		err := os.Rename(j.StagingPath, newPath)

		if err != nil {
			slog.Error("move failed", "file", newPath, "err", err)
			os.Remove(j.StagingPath)
			c.idx.Remove(id)
		}

	}
	return nil
}
