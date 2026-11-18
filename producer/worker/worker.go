package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	videouploadv1 "github.com/zrygan/media-upload-service/gen/videoupload/v1"
	"github.com/zrygan/media-upload-service/producer/network"
	"github.com/zrygan/media-upload-service/producer/upload"
)

// videoExts lists the lowercase file extensions a worker uploads. It is
// never written after init, so all workers can read it without a lock.
var videoExts = map[string]bool{
	".mp4": true, ".m4v": true, ".mov": true,
	".mkv": true, ".webm": true, ".avi": true,
	".wmv": true, ".flv": true, ".mpg": true,
	".mpeg": true, ".3gp": true, ".ogv": true,
	".ts": true,
}

// RunSummary holds the upload counts of one ProducerWorker. numSent
// counts every upload that was attempted, so it is always the sum of
// the other four counts.
type RunSummary struct {
	numSent      int
	numAccepted  int
	numDropped   int
	numDuplicate int
	numFailed    int
}

// ProducerWorker uploads every video in one folder to the consumer. Each
// ProducerWorker runs on its own goroutine/thread, see RunThreads.
type ProducerWorker struct {
	id         int
	folderName string
	rs         RunSummary
}

// run uploads each video in the worker's folder, then logs the worker's
// RunSummary. Subfolders and files whose extension is not in videoExts
// are skipped and not counted as sent. A failed upload is logged and
// counted, and the worker continues with the next file. If ctx is
// cancelled, the worker stops before starting another upload.
//
// This returns an error only if the folder cannot be read.
func (pw *ProducerWorker) run(ctx context.Context, client *network.Client) error {

	folder, err := os.ReadDir(pw.folderName)
	if err != nil {
		return fmt.Errorf("worker %d could not open its assigned folder %s: %w",
			pw.id, pw.folderName, err,
		)
	}
	for _, video := range folder {
		// the producer is shutting down, do not start another upload
		if ctx.Err() != nil {
			break
		}

		if video.IsDir() {
			continue
		}

		vidExt := filepath.Ext(video.Name())

		if !videoExts[strings.ToLower(vidExt)] {
			slog.Info("skipped non-video file",
				"component", "worker",
				"worker_id", pw.id,
				"file", video.Name(),
				"ext", vidExt,
			)
			continue
		}

		pw.rs.numSent++
		status, err := upload.File(
			ctx,
			client,
			pw.id,
			filepath.Join(pw.folderName, video.Name()))

		// a failed file should not stop the rest of the folder
		if err != nil {
			pw.rs.numFailed++
			slog.Error("upload failed",
				"component", "worker",
				"worker_id", pw.id,
				"file", video.Name(),
				"error", err,
			)
			continue
		}

		// logging
		level := slog.LevelWarn
		if status == videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED {
			level = slog.LevelInfo
		}
		slog.Log(ctx, level, "upload finished",
			"component", "worker",
			"worker_id", pw.id,
			"file", video.Name(),
			"status", status.String(),
		)

		// run summary tracking
		pw.rs.update(status)
	}

	slog.Info("worker finished",
		"component", "worker",
		"worker_id", pw.id,
		"sent", pw.rs.numSent,
		"accepted", pw.rs.numAccepted,
		"dropped", pw.rs.numDropped,
		"duplicate", pw.rs.numDuplicate,
		"failed", pw.rs.numFailed,
	)

	return nil
}

// update adds a finished upload's status to the counts.
func (rs *RunSummary) update(uploadStatus videouploadv1.VideoUploadStatus) {
	switch uploadStatus {
	case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_STATUS_UNSPECIFIED:
		rs.numFailed++
	case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED:
		rs.numAccepted++
	case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_QUEUE_FULL:
		rs.numDropped++
	case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_DUPLICATE:
		rs.numDuplicate++
	case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ERROR:
		rs.numFailed++
	}
}
