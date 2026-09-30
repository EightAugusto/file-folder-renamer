package apply

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

type ApplyError struct {
	Cause          error
	RollbackErrors []error
}

func (e *ApplyError) Error() string {
	if len(e.RollbackErrors) == 0 {
		return fmt.Sprintf("apply renames: %v; all completed operations were rolled back", e.Cause)
	}
	return fmt.Sprintf("apply renames: %v; rollback was incomplete (%d errors)", e.Cause, len(e.RollbackErrors))
}

func (e *ApplyError) Unwrap() error { return e.Cause }

type fileOps struct {
	lstat      func(string) (os.FileInfo, error)
	rename     func(string, string) error
	randomName func() (string, error)
}

type renameOperation struct{ from, to string }

type stagedProposal struct {
	proposal rename.Proposal
	tempPath string
}

// Validate rejects malformed paths and targets that collide within a batch.
func Validate(proposals []rename.Proposal) error {
	seenSources := make(map[string]struct{})
	seenTargets := make(map[string]rename.Proposal)
	for _, item := range proposals {
		if item.Kind != rename.NodeKindFile && item.Kind != rename.NodeKindFolder {
			return fmt.Errorf("proposal %q has unknown node kind %q", item.SourcePath, item.Kind)
		}
		sourcePath := filepath.Clean(item.SourcePath)
		parentDir := filepath.Clean(item.ParentDir)
		if sourcePath == "." || !filepath.IsAbs(sourcePath) {
			return fmt.Errorf("proposal source %q must be absolute", item.SourcePath)
		}
		if !filepath.IsAbs(parentDir) || filepath.Dir(sourcePath) != parentDir {
			return fmt.Errorf("proposal source %q is not inside parent %q", item.SourcePath, item.ParentDir)
		}
		if _, duplicate := seenSources[sourcePath]; duplicate {
			return fmt.Errorf("duplicate proposal source %q", item.SourcePath)
		}
		seenSources[sourcePath] = struct{}{}
		if item.ProposedName == "" || item.ProposedName == "." || item.ProposedName == ".." || filepath.IsAbs(item.ProposedName) || filepath.Base(item.ProposedName) != item.ProposedName || strings.ContainsAny(item.ProposedName, `/\`) {
			return fmt.Errorf("proposal target %q must be a non-empty base name", item.ProposedName)
		}
		proposedPath := filepath.Clean(filepath.Join(item.ParentDir, item.ProposedName))
		if previous, found := seenTargets[proposedPath]; found {
			return fmt.Errorf("collision: %q and %q both propose %q in %q", previous.SourcePath, item.SourcePath, item.ProposedName, item.ParentDir)
		}
		seenTargets[proposedPath] = item
	}
	return nil
}

// ValidateFilesystem verifies that every source still exists and that a final
// target is either free, the source itself, or another source in this batch.
func ValidateFilesystem(proposals []rename.Proposal) error {
	return validateFilesystem(proposals, productionFileOps())
}

func validateFilesystem(proposals []rename.Proposal, fs fileOps) error {
	fs = withFileOpDefaults(fs)
	if err := Validate(proposals); err != nil {
		return err
	}
	changed := changedProposals(proposals)
	// Resolve sources first so a target occupied by another batch source is safe.
	sourceInfo := make(map[string]os.FileInfo, len(changed))
	for _, item := range changed {
		info, err := fs.lstat(item.SourcePath)
		if err != nil {
			return fmt.Errorf("source %q is unavailable: %w", item.SourcePath, err)
		}
		sourceInfo[filepath.Clean(item.SourcePath)] = info
	}

	for _, item := range changed {
		target := filepath.Clean(filepath.Join(item.ParentDir, item.ProposedName))
		targetInfo, err := fs.lstat(target)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect target %q: %w", target, err)
		}
		if os.SameFile(sourceInfo[filepath.Clean(item.SourcePath)], targetInfo) {
			continue
		}
		occupiedByBatchSource := false
		for sourcePath, info := range sourceInfo {
			if sourcePath != filepath.Clean(item.SourcePath) && os.SameFile(info, targetInfo) {
				occupiedByBatchSource = true
				break
			}
		}
		if !occupiedByBatchSource {
			return fmt.Errorf("target %q already exists and is not being renamed", target)
		}
	}
	return nil
}

func Apply(proposals []rename.Proposal) error {
	return applyWithOps(proposals, productionFileOps())
}

func applyWithOps(proposals []rename.Proposal, fs fileOps) error {
	fs = withFileOpDefaults(fs)
	if err := validateFilesystem(proposals, fs); err != nil {
		return err
	}
	changed := changedProposals(proposals)
	journal := make([]renameOperation, 0, len(changed)*2)

	for start := 0; start < len(changed); {
		// Stage siblings together so chains, swaps, and case-only renames can finish.
		end := start + 1
		for end < len(changed) && changed[end].Depth == changed[start].Depth && changed[end].ParentDir == changed[start].ParentDir {
			end++
		}
		staged := make([]stagedProposal, 0, end-start)
		for _, item := range changed[start:end] {
			tempPath, err := unusedTempPath(item.ParentDir, fs)
			if err != nil {
				return rollbackFailure(err, journal, fs)
			}
			if err := fs.rename(item.SourcePath, tempPath); err != nil {
				return rollbackFailure(fmt.Errorf("stage %q: %w", item.SourcePath, err), journal, fs)
			}
			journal = append(journal, renameOperation{from: item.SourcePath, to: tempPath})
			staged = append(staged, stagedProposal{proposal: item, tempPath: tempPath})
		}
		for _, stagedItem := range staged {
			// Recheck targets immediately before promotion; another process may have
			// created one after the initial filesystem validation.
			target := filepath.Join(stagedItem.proposal.ParentDir, stagedItem.proposal.ProposedName)
			if _, err := fs.lstat(target); err == nil {
				return rollbackFailure(fmt.Errorf("target %q became occupied before rename", target), journal, fs)
			} else if !errors.Is(err, os.ErrNotExist) {
				return rollbackFailure(fmt.Errorf("inspect target %q: %w", target, err), journal, fs)
			}
			if err := fs.rename(stagedItem.tempPath, target); err != nil {
				return rollbackFailure(fmt.Errorf("finalize %q: %w", target, err), journal, fs)
			}
			journal = append(journal, renameOperation{from: stagedItem.tempPath, to: target})
		}
		start = end
	}
	return nil
}

func changedProposals(proposals []rename.Proposal) []rename.Proposal {
	changed := make([]rename.Proposal, 0, len(proposals))
	for _, item := range proposals {
		if item.Changed {
			changed = append(changed, item)
		}
	}
	// Rename deeper entries before their parent folders change paths.
	sort.SliceStable(changed, func(i, j int) bool {
		if changed[i].Depth != changed[j].Depth {
			return changed[i].Depth > changed[j].Depth
		}
		if changed[i].ParentDir != changed[j].ParentDir {
			return strings.Compare(changed[i].ParentDir, changed[j].ParentDir) < 0
		}
		return strings.Compare(changed[i].SourcePath, changed[j].SourcePath) < 0
	})
	return changed
}

func productionFileOps() fileOps {
	return fileOps{lstat: os.Lstat, rename: os.Rename, randomName: randomTemporaryName}
}

func withFileOpDefaults(fs fileOps) fileOps {
	defaults := productionFileOps()
	if fs.lstat == nil {
		fs.lstat = defaults.lstat
	}
	if fs.rename == nil {
		fs.rename = defaults.rename
	}
	if fs.randomName == nil {
		fs.randomName = defaults.randomName
	}
	return fs
}

func randomTemporaryName() (string, error) {
	return randomTemporaryNameFrom(rand.Reader)
}

func randomTemporaryNameFrom(reader io.Reader) (string, error) {
	var random [8]byte
	if _, err := io.ReadFull(reader, random[:]); err != nil {
		return "", fmt.Errorf("generate temporary rename: %w", err)
	}
	return ".file-folder-renamer-" + hex.EncodeToString(random[:]), nil
}

func unusedTempPath(parent string, fs fileOps) (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		name, err := fs.randomName()
		if err != nil {
			return "", err
		}
		if name == "" || filepath.Base(name) != name {
			return "", fmt.Errorf("generate temporary rename: invalid name %q", name)
		}
		candidate := filepath.Join(parent, name)
		if _, err := fs.lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect temporary path %q: %w", candidate, err)
		}
	}
	return "", errors.New("could not allocate a temporary rename path")
}

func rollbackFailure(cause error, journal []renameOperation, fs fileOps) error {
	// Undo completed moves in reverse order so each earlier source path is free.
	rollbackErrors := make([]error, 0)
	for index := len(journal) - 1; index >= 0; index-- {
		op := journal[index]
		if _, err := fs.lstat(op.from); err == nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %q: destination became occupied", op.from))
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("inspect rollback destination %q: %w", op.from, err))
			continue
		}
		if err := fs.rename(op.to, op.from); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %q from %q: %w", op.from, op.to, err))
		}
	}
	return &ApplyError{Cause: cause, RollbackErrors: rollbackErrors}
}
