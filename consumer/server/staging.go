package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// StagingPrefix is the start of every temp file name that UploadVideo
// creates in the staging folder. CleanStaging uses the same prefix to
// find leftovers, so the two always agree on which files are ours.
const StagingPrefix = "upload-"
const StagingExt = ".part"

// CleanStaging removes leftover StagingPrefix files from dir, such as
// uploads that were cut off when the consumer last stopped. It returns
// how many files it removed.
//
// Call it before the gRPC server starts. While the server runs, the
// staging folder holds uploads that are still arriving, and removing
// one would break that upload.
//
// Only regular files that start with StagingPrefix are removed, so a
// misconfigured folder (for example the media folder) is left alone.
// A file that cannot be removed is logged and skipped. The returned
// error is only for when dir itself cannot be read.
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
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			slog.Warn("could not remove staging file", "component", "server", "file", e.Name(), "error", err)
			continue // one stuck file shouldn't stop startup
		}
		removed++
	}
	return removed, nil
}

func tempFile(dir string, sum []byte) (string, error) {
	if len(sum) != sha256.Size {
		return "", fmt.Errorf("checksum must be %d bytes, got %d", sha256.Size, len(sum))
	}
	return filepath.Join(dir, StagingPrefix+hex.EncodeToString(sum)+StagingExt), nil
}
