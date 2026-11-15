package worker

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zrygan/media-upload-service/consumer/inflight"
	"github.com/zrygan/media-upload-service/consumer/queue"
)

// ConsumerWorker moves finished uploads from staging into the media
// folder. Each one runs on its own goroutine (see RunThreads).
type ConsumerWorker struct {
	id    int
	q     *queue.Queue
	media string
	mut   *sync.Mutex
	h     *inflight.Hashbrowns
}

// run handles jobs until the queue is closed and empty. A failed job is
// logged and skipped. It always returns nil. Each job is received by
// exactly one worker, so taking it needs no lock.
func (c *ConsumerWorker) run() error {
	js := c.q.Jobs()
	for {

		j, ok := <-js

		if !ok {
			break
		}

		// the job left the queue, so its slot is free again
		c.q.UnReserve()

		if err := c.handle(j); err != nil {
			slog.Error("job failed", "file", j.FileName, "err", err)
		}

		// done with this video either way, so new uploads of it can start
		c.h.Drop(hex.EncodeToString(j.Hash))

	}
	return nil
}

// handle renames the staged file into the media folder, under its
// original name or "name (n).ext" if that is taken. A rename is
// atomic, so the GUI never sees a half-written video (staging and
// media must share a filesystem). If the rename fails, the staged file
// is removed.
//
// mut is held from the name check to the rename. Without it, two
// workers could pick the same free name and one video would overwrite
// the other.
func (c *ConsumerWorker) handle(j queue.Job) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	fName := filepath.Base(j.FileName)
	ext := filepath.Ext(fName)
	base := strings.TrimSuffix(fName, ext)

	for n := 0; ; n++ {
		candidate := fName
		if n > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", base, n, ext)
		}
		dst := filepath.Join(c.media, candidate)

		_, err := os.Stat(dst)
		if err == nil {
			continue // name taken, try the next number
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("checking %s: %w", dst, err)
		}

		if err := os.Rename(j.StagingPath, dst); err != nil {
			os.Remove(j.StagingPath)
			return fmt.Errorf("moving to %s: %w", dst, err)
		}
		return nil
	}
}
