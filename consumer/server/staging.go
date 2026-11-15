package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StagingPrefix is the start of every temp file name that UploadVideo
// creates in the staging folder. CleanStaging uses the same prefix to
// find leftovers, so the two always agree on which files are ours.
const StagingPrefix = "upload-"

// StagingExt is the end of every staged file name built by tempFile. A
// .part file is an upload that has not finished yet, which a producer
// can resume.
const StagingExt = ".part"

// stagingMaxAge is how long an unfinished upload (a .part file) is kept
// after it was last written to. A producer that comes back within this
// time can resume. An older .part file is treated as abandoned and
// CleanStaging removes it.
const stagingMaxAge = 24 * time.Hour

// CleanStaging removes leftover StagingPrefix files from dir. It
// returns how many files it removed.
//
//   - A .part file (an unfinished upload) is kept if it was written to
//     within stagingMaxAge, so its producer can still resume it. An
//     older one is removed as abandoned.
//   - Any other StagingPrefix file is removed right away. Those are
//     left over from older versions of the consumer, which used random
//     staging names that nothing can resume.
//
// Call it before the gRPC server starts. While the server runs, the
// staging folder holds uploads that are still arriving, and removing
// one would break that upload.
//
// Only regular files that start with StagingPrefix are removed, so a
// misconfigured folder (for example the media folder) is left alone.
// A file that cannot be checked or removed is logged and skipped. The
// returned error is only for when dir itself cannot be read.
func CleanStaging(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), StagingPrefix) {
			continue // not ours: leave it alone
		}

		info, err := e.Info()
		if err != nil {
			slog.Warn("could not check staging file", "component", "server", "file", e.Name(), "error", err)
			continue // same as a failed remove: skip it, keep going
		}

		// a recent .part file is an upload a producer can still resume.
		// ModTime is the last write, so an upload that keeps resuming
		// stays recent
		if strings.HasSuffix(e.Name(), StagingExt) && time.Since(info.ModTime()) < stagingMaxAge {
			continue
		}

		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			slog.Warn("could not remove staging file", "component", "server", "file", e.Name(), "error", err)
			continue // one stuck file shouldn't stop startup
		}
		removed++
	}
	return removed, nil
}

// tempFile returns the staging path for the video with SHA-256 hash
// sum: dir/upload-<hash in hex>.part. The name comes only from the
// hash, so every attempt to upload the same video finds the same file.
// This is what lets an upload resume. Every place that needs a staged
// file's name must build it here, so they always agree.
//
// The hash comes from the producer, so its length is checked. Without
// the check, an empty hash would become "upload-.part", and every
// upload with an empty hash would share that one file.
func tempFile(dir string, sum []byte) (string, error) {
	if len(sum) != sha256.Size {
		return "", fmt.Errorf("checksum must be %d bytes, got %d", sha256.Size, len(sum))
	}
	return filepath.Join(dir, StagingPrefix+hex.EncodeToString(sum)+StagingExt), nil
}
