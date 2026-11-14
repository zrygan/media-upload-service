package worker

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zrygan/media-upload-service/consumer/queue"
)

// ConsumerWorker moves finished uploads from the staging folder into
// the media folder. Each ConsumerWorker runs on its own
// goroutine/thread, see RunThreads.
type ConsumerWorker struct {
	id    int
	q     *queue.Queue
	media string
	mut   *sync.Mutex
}

// run takes jobs off the queue until the queue is closed and empty,
// and moves each staged file into the media folder. A failed move is
// logged, the staged file is removed, and the worker continues with
// the next job. This currently always returns nil.
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

		c.q.UnReserve()

		if err := c.handle(j); err != nil {
			slog.Error("job failed", "file", j.FileName, "err", err)
		}

	}
	return nil
}

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
