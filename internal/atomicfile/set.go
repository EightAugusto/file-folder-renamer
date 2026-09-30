package atomicfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type stagedFile struct {
	path, temporary, backup string
	promoted                bool
}

// ReplaceJSONSet atomically writes values and removes obsolete paths as one
// logical transaction. Existing files are restored if any promotion fails.
func ReplaceJSONSet(values map[string]any, obsolete []string) (err error) {
	return replaceJSONSet(values, obsolete, productionOperations())
}

func replaceJSONSet(values map[string]any, obsolete []string, ops fileOperations) (err error) {
	committed := false
	// Stage every new document before touching an existing destination.
	paths := make([]string, 0, len(values))
	for path := range values {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	staged := make([]stagedFile, 0, len(paths))
	defer func() {
		for _, file := range staged {
			for _, path := range []string{file.temporary, file.backup} {
				if path == "" {
					continue
				}
				if removeErr := ops.remove(path); !committed && removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					err = errors.Join(err, removeErr)
				}
			}
		}
	}()

	for _, path := range paths {
		directory := filepath.Dir(path)
		if err := ops.mkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create parent directory %q: %w", directory, err)
		}
		data, err := json.MarshalIndent(values[path], "", "  ")
		if err != nil {
			return fmt.Errorf("encode %q: %w", path, err)
		}
		temporary, err := ops.createTemp(directory, ".file-folder-renamer-*.tmp")
		if err != nil {
			return fmt.Errorf("create temporary file for %q: %w", path, err)
		}
		temporaryPath := temporary.Name()
		data = append(data, '\n')
		written, writeErr := temporary.Write(data)
		if writeErr == nil && written != len(data) {
			writeErr = io.ErrShortWrite
		}
		if writeErr != nil {
			return discardTemporary(temporary, temporaryPath, true, fmt.Errorf("write temporary file for %q: %w", path, writeErr), ops)
		}
		if err := temporary.Sync(); err != nil {
			return discardTemporary(temporary, temporaryPath, true, fmt.Errorf("sync temporary file for %q: %w", path, err), ops)
		}
		if err := temporary.Close(); err != nil {
			return discardTemporary(temporary, temporaryPath, false, fmt.Errorf("close temporary file for %q: %w", path, err), ops)
		}
		staged = append(staged, stagedFile{path: path, temporary: temporaryPath})
	}
	// Validate the complete write and deletion set before making backups.
	touched := make(map[string]struct{}, len(staged)+len(obsolete))
	for _, file := range staged {
		touched[file.path] = struct{}{}
	}
	for _, path := range obsolete {
		if _, writing := touched[path]; writing {
			continue
		}
		touched[path] = struct{}{}
		staged = append(staged, stagedFile{path: path})
	}
	sort.Slice(staged, func(left, right int) bool { return staged[left].path < staged[right].path })
	for _, file := range staged {
		info, statErr := ops.lstat(file.path)
		if statErr == nil && info.IsDir() {
			return fmt.Errorf("replace %q: destination is a directory", file.path)
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("inspect %q: %w", file.path, statErr)
		}
	}

	// Back up all existing paths, including paths scheduled for deletion.
	for index := range staged {
		if _, statErr := ops.lstat(staged[index].path); statErr == nil {
			backup, err := unusedPath(filepath.Dir(staged[index].path), ops)
			if err != nil {
				return rollbackSet(staged, err, ops)
			}
			if err := ops.rename(staged[index].path, backup); err != nil {
				return rollbackSet(staged, fmt.Errorf("back up %q: %w", staged[index].path, err), ops)
			}
			staged[index].backup = backup
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return rollbackSet(staged, fmt.Errorf("inspect %q: %w", staged[index].path, statErr), ops)
		}
	}
	// Promote staged documents only after every backup is in place.
	for index := range staged {
		if staged[index].temporary == "" {
			continue
		}
		if err := ops.rename(staged[index].temporary, staged[index].path); err != nil {
			return rollbackSet(staged, fmt.Errorf("replace %q: %w", staged[index].path, err), ops)
		}
		staged[index].temporary = ""
		staged[index].promoted = true
	}
	committed = true
	return nil
}

func unusedPath(directory string, ops fileOperations) (string, error) {
	file, err := ops.createTemp(directory, ".file-folder-renamer-backup-*.tmp")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", discardTemporary(file, path, false, err, ops)
	}
	if err := ops.remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func discardTemporary(file temporaryFile, path string, closeFile bool, cause error, ops fileOperations) error {
	errs := []error{cause}
	if closeFile {
		if err := file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close temporary file %q: %w", path, err))
		}
	}
	if err := ops.remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove temporary file %q: %w", path, err))
	}
	return errors.Join(errs...)
}

func rollbackSet(staged []stagedFile, cause error, ops fileOperations) error {
	// Reverse promotion and backup moves; preserve any backup that cannot be
	// restored so the old document is still recoverable.
	errs := []error{cause}
	for index := len(staged) - 1; index >= 0; index-- {
		file := staged[index]
		if file.promoted {
			if err := ops.remove(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, fmt.Errorf("remove incomplete %q: %w", file.path, err))
			}
		}
		if file.backup != "" {
			// Once restoration is attempted, deferred cleanup must not remove the
			// backup. On failure it is the only remaining copy of the old file.
			staged[index].backup = ""
			if err := ops.rename(file.backup, file.path); err != nil {
				errs = append(errs, fmt.Errorf("restore %q: %w", file.path, err))
			}
		}
	}
	return errors.Join(errs...)
}
