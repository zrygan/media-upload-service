package upload

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// SHA256 is a wrapper function that calls the standard libraries
// implementation of the SHA256 (via Sum256) but takes in a video of
// type *os.File instead of an array of bytes.
func SHA256(video *os.File) ([32]byte, error) {
	if _, err := video.Seek(0, io.SeekStart); err != nil {
		return [32]byte{}, fmt.Errorf("failed to seek to start of %s: %w", video.Name(), err)
	}

	hasher := sha256.New()

	if _, err := io.Copy(hasher, video); err != nil {
		return [32]byte{}, fmt.Errorf("SHA256 failed to stream %s: %w", video.Name(), err)
	}

	var result [32]byte
	copy(result[:], hasher.Sum(nil))

	return result, nil
}
