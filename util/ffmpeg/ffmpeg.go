package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Run calls ffmpeg with args and returns what it wrote to stdout. The
// returned error includes what ffmpeg wrote to stderr, and ffmpeg is
// stopped if ctx is canceled.
func Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg finished with error: %w (stderr: %s)", err, stderr.String())
	}

	return stdout, nil
}
