package util

import (
	"fmt"
	"os"
)

// CheckDirReadable is a utility function that returns an Error if the
// folder with dirName does not exist or cannot be accessed.
func CheckDirReadable(dirName string) error {
	if _, err := os.ReadDir(dirName); err != nil {
		return fmt.Errorf("cannot open folder %s: %w", dirName, err)
	}
	return nil
}

// CheckDirWritable is a utility function that returns an Error if the
// program does not have write permissions to the folder with name
// dirName.
func CheckDirWritable(dirName string) error {
	tempf, err := os.CreateTemp(dirName, "check-perms-*.tmp")
	if err != nil {
		return fmt.Errorf("cannot write to folder %s: %w", dirName, err)
	}

	tempf.Close()
	if err := os.Remove(tempf.Name()); err != nil {
		return fmt.Errorf("cannot remove temp file %s: %w", tempf.Name(), err)
	}
	return nil
}
