// Package atomicfile writes complete files without exposing partial contents.
package atomicfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteJSON encodes value into a temporary file beside path, syncs it, and
// atomically replaces path. Temporary files are removed on every failure.
func WriteJSON(path string, value any) (err error) {
	return writeJSON(path, value, productionOperations())
}

func writeJSON(path string, value any, ops fileOperations) (err error) {
	directory := filepath.Dir(path)
	if err := ops.mkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create parent directory %q: %w", directory, err)
	}
	temporary, err := ops.createTemp(directory, ".file-folder-renamer-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file for %q: %w", path, err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, temporary.Close())
		}
		if removeErr := ops.remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode %q: %w", path, err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync %q: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %q: %w", path, err)
	}
	closed = true
	if err := ops.rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %q: %w", path, err)
	}
	return nil
}
