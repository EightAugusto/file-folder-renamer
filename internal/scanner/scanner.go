package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

type CollectOptions struct {
	Files        bool
	Folders      bool
	IgnoredNames []string
}

type ignoredMatcher []string

type scanOperations struct {
	abs     func(string) (string, error)
	lstat   func(string) (os.FileInfo, error)
	readDir func(string) ([]os.DirEntry, error)
}

func CollectContext(ctx context.Context, root string, opts CollectOptions) ([]rename.Entry, error) {
	return collectContext(ctx, root, opts, scanOperations{abs: filepath.Abs, lstat: os.Lstat, readDir: os.ReadDir})
}

func collectContext(ctx context.Context, root string, opts CollectOptions, ops scanOperations) ([]rename.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !opts.Files && !opts.Folders {
		return nil, errors.New("at least one of files or folders must be enabled")
	}
	ignoredNames, err := compileIgnoredNames(opts.IgnoredNames)
	if err != nil {
		return nil, err
	}

	absRoot, err := ops.abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve scan root %q: %w", root, err)
	}
	rootInfo, err := ops.lstat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect scan root %q: %w", absRoot, err)
	}
	if !rootInfo.IsDir() {
		if !opts.Files {
			return nil, errors.New("selected path is a file but files are not included")
		}
		if ignoredNames.matches(filepath.Base(absRoot)) {
			return nil, nil
		}
		return []rename.Entry{{
			SourcePath: absRoot, Name: filepath.Base(absRoot), Kind: rename.NodeKindFile, Depth: 0,
		}}, nil
	}

	var entries []rename.Entry
	if opts.Folders {
		entries = append(entries, rename.Entry{
			SourcePath: absRoot,
			Name:       filepath.Base(absRoot),
			Kind:       rename.NodeKindFolder,
			Depth:      0,
		})
	}

	err = walk(ctx, absRoot, 0, opts, ignoredNames, ops, &entries)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func walk(ctx context.Context, current string, depth int, opts CollectOptions, ignoredNames ignoredMatcher, ops scanOperations, entries *[]rename.Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	childrenEntries, err := ops.readDir(current)
	if err != nil {
		return fmt.Errorf("read directory %q: %w", current, err)
	}
	sort.Slice(childrenEntries, func(leftIndex, rightIndex int) bool {
		return childrenEntries[leftIndex].Name() < childrenEntries[rightIndex].Name()
	})

	for _, childEntry := range childrenEntries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ignoredNames.matches(childEntry.Name()) {
			continue
		}
		sourcePath := filepath.Join(current, childEntry.Name())
		kind := rename.NodeKindFile
		if childEntry.IsDir() {
			kind = rename.NodeKindFolder
		}

		if (kind == rename.NodeKindFile && opts.Files) || (kind == rename.NodeKindFolder && opts.Folders) {
			*entries = append(*entries, rename.Entry{
				SourcePath: sourcePath,
				Name:       childEntry.Name(),
				Kind:       kind,
				Depth:      depth + 1,
			})
		}

		if childEntry.IsDir() {
			if err := walk(ctx, sourcePath, depth+1, opts, ignoredNames, ops, entries); err != nil {
				return err
			}
		}
	}
	return nil
}

func compileIgnoredNames(patterns []string) (ignoredMatcher, error) {
	compiled := make(ignoredMatcher, len(patterns))
	for index, pattern := range patterns {
		if _, err := filepath.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("invalid ignored name pattern %q: %w", pattern, err)
		}
		compiled[index] = strings.ToLower(pattern)
	}
	return compiled, nil
}

func (matcher ignoredMatcher) matches(name string) bool {
	name = strings.ToLower(name)
	for _, pattern := range matcher {
		matched, _ := filepath.Match(pattern, name) // patterns were validated once
		if matched {
			return true
		}
	}
	return false
}
