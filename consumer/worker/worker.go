package worker

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zrygan/media-upload-service/consumer/dedup"
	"github.com/zrygan/media-upload-service/consumer/inflight"
	"github.com/zrygan/media-upload-service/consumer/queue"
	"github.com/zrygan/media-upload-service/consumer/server"
	"github.com/zrygan/media-upload-service/util/ffmpeg"
)

// compressTimeout is the longest one ffmpeg run may take. After that
// the original is saved instead.
const compressTimeout = 2 * time.Minute

// ConsumerWorker moves finished uploads from staging into the media
// folder. Each one runs on its own goroutine (see RunThreads).
type ConsumerWorker struct {
	id    int
	q     *queue.Queue
	idx   *dedup.Index
	media string
	mut   *sync.Mutex
	h     *inflight.Hashbrowns
}

// run handles jobs until the queue is closed and empty. A failed job is
// logged, removed from the duplicate index, and skipped. It always
// returns nil. Each job is received by exactly one worker, so taking it
// needs no lock.
func (c *ConsumerWorker) run(ctx context.Context) error {
	js := c.q.Jobs()
	for {

		j, ok := <-js

		if !ok {
			break
		}

		// the job left the queue, so its slot is free again
		c.q.UnReserve()

		id := hex.EncodeToString(j.Hash)
		if err := c.handle(ctx, j); err != nil {
			slog.Error("job failed", "file", j.FileName, "err", err)
			c.idx.Remove(id)
		}

		// done with this video either way, so new uploads of it can start
		c.h.Drop(id)

	}
	return nil
}

// handle compresses the staged file (see pick), then renames the
// smaller copy into the media folder, under its original name or
// "name (n).ext" if that is taken. A compressed copy is always saved as
// .mp4. A rename is atomic, so the GUI never sees a half-written video
// (staging and media must share a filesystem). The other copy is
// removed only after the rename, so a crash before it loses nothing. If
// the rename fails, both are removed.
//
// Compression runs before mut is taken, so workers compress in
// parallel. mut is held only from the name check to the rename.
// Without it, two workers could pick the same free name and one video
// would overwrite the other.
func (c *ConsumerWorker) handle(ctx context.Context, j queue.Job) error {
	// staging/upload-<hex>.part -> staging/compress-<hex>.mp4, unique
	// per job, so workers never share an output file
	hash := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(j.StagingPath), server.StagingPrefix), server.StagingExt)
	compressPath := filepath.Join(filepath.Dir(j.StagingPath), server.CompressPrefix+hash+".mp4")

	jobCtx, cancel := context.WithTimeout(ctx, compressTimeout)
	defer cancel()

	start := time.Now()
	keep, discard := c.pick(jobCtx, j.StagingPath, compressPath)
	compressed := keep == compressPath

	c.mut.Lock()
	defer c.mut.Unlock()

	fName := filepath.Base(j.FileName)
	ext := filepath.Ext(fName)
	base := strings.TrimSuffix(fName, ext)

	if compressed {
		ext = ".mp4"
		fName = base + ext
	}

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

		if err := os.Rename(keep, dst); err != nil {
			os.Remove(keep)
			os.Remove(discard)
			return fmt.Errorf("moving to %s: %w", dst, err)
		}
		os.Remove(discard)

		saved := j.FileSize
		if info, err := os.Stat(dst); err == nil {
			saved = info.Size()
		}
		slog.Info("video saved", "worker_id", c.id, "file", candidate, "compressed", compressed,
			"original_bytes", j.FileSize, "saved_bytes", saved, "took", time.Since(start).Round(time.Millisecond).String())
		return nil
	}
}

// compress re-encodes in to H.264/AAC at most 720p tall and writes it
// to out, which must end in .mp4. faststart puts the index first so the
// browser can start playing before the whole file arrives. ffmpeg is
// killed if ctx ends.
func (c *ConsumerWorker) compress(ctx context.Context, in, out string) error {
	_, err := ffmpeg.Run(ctx, "-y", "-v", "error", "-i", in,
		"-c:v", "libx264", "-crf", "28", "-preset", "veryfast",
		"-vf", "scale=-2:'min(720,ih)'",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "96k",
		"-movflags", "+faststart",
		out)
	return err
}

// pick compresses in to out and reports which file to keep and which
// to discard. It keeps out only if ffmpeg succeeded and out is smaller,
// so a failed, canceled or useless compression always keeps the
// original. It deletes nothing (see handle).
func (c *ConsumerWorker) pick(ctx context.Context, in, out string) (keep, discard string) {
	if err := c.compress(ctx, in, out); err != nil {
		slog.Warn("compression failed, keeping original", "worker_id", c.id, "file", filepath.Base(in), "err", err)
		return in, out // out is partial or missing, never use it
	}
	a, errA := os.Stat(in)
	b, errB := os.Stat(out)
	if errA != nil || errB != nil || b.Size() >= a.Size() {
		return in, out // can't compare, or not worth it
	}
	return out, in
}
