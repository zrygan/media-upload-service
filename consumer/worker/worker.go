package worker

import (
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/zrygan/media-upload-service/consumer/queue"
)

type ConsumerWorker struct {
	id    int
	q     *queue.Queue
	media string
}

func (c *ConsumerWorker) run() error {
	js := c.q.Jobs()
	for {

		j, ok := <-js

		if !ok {
			break
		}
		c.q.UnReserve()

		//maybe change hash join later
		newName := hex.EncodeToString(j.Hash) + "_" + filepath.Base(j.FileName)
		newPath := filepath.Join(c.media, newName)

		err := os.Rename(j.StagingPath, newPath)

		if err != nil {
			slog.Error("move failed", "file", newPath, "err", err)
			os.Remove(j.StagingPath)
		}

	}
	return nil
}
