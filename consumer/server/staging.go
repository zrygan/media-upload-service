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

// StagingPrefix starts every upload's staging file name. CleanStaging
// only touches files that start with it or with CompressPrefix.
const StagingPrefix = "upload-"

// CompressPrefix starts the name of a worker's compressed copy while
// ffmpeg writes it. Nothing resumes from these, so CleanStaging always
// removes them.
const CompressPrefix = "compress-"

// StagingExt ends the name of an unfinished upload that can be resumed.
const StagingExt = ".part"

// stagingMaxAge is how long an untouched .part file is kept before
// CleanStaging treats it as abandoned.
const stagingMaxAge = 24 * time.Hour

// CleanStaging removes leftovers from dir and returns how many it
// removed: .part files not written to within stagingMaxAge, any other
// StagingPrefix file (random names from older versions), and every
// CompressPrefix file (from a crash while compressing). Recent .part
// files stay so they can resume. Files with neither prefix are never
// touched, in case dir is misconfigured.
//
// Call it before the server starts, so it can't remove a live upload.
// The error is only for when dir can't be read.
func CleanStaging(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, e := range entries {
		ours := strings.HasPrefix(e.Name(), StagingPrefix) || strings.HasPrefix(e.Name(), CompressPrefix)
		if e.IsDir() || !ours {
			continue // not ours: leave it alone
		}

		info, err := e.Info()
		if err != nil {
			slog.Warn("could not check staging file", "component", "server", "file", e.Name(), "error", err)
			continue
		}

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

// tempFile returns dir/upload-<hex hash>.part. The name depends only on
// the hash, so every attempt for the same video finds the same file.
// Build staging names only here. The length check stops an empty hash
// from making every bad upload share "upload-.part".
func tempFile(dir string, sum []byte) (string, error) {
	if len(sum) != sha256.Size {
		return "", fmt.Errorf("checksum must be %d bytes, got %d", sha256.Size, len(sum))
	}
	return filepath.Join(dir, StagingPrefix+hex.EncodeToString(sum)+StagingExt), nil
}
