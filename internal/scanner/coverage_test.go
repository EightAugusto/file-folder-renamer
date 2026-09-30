package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

func TestCollectInjectedFilesystemFailures(t *testing.T) {
	sentinel := errors.New("failure")
	root := t.TempDir()
	base := scanOperations{abs: filepath.Abs, lstat: os.Lstat, readDir: os.ReadDir}
	ops := base
	ops.abs = func(string) (string, error) { return "", sentinel }
	if _, err := collectContext(context.Background(), root, CollectOptions{Files: true}, ops); !errors.Is(err, sentinel) {
		t.Fatalf("absolute: %v", err)
	}
	ops = base
	ops.lstat = func(string) (os.FileInfo, error) { return nil, sentinel }
	if _, err := collectContext(context.Background(), root, CollectOptions{Files: true}, ops); !errors.Is(err, sentinel) {
		t.Fatalf("stat: %v", err)
	}
	ops = base
	ops.readDir = func(string) ([]os.DirEntry, error) { return nil, sentinel }
	if _, err := collectContext(context.Background(), root, CollectOptions{Files: true}, ops); !errors.Is(err, sentinel) {
		t.Fatalf("read dir: %v", err)
	}
}

func TestCollectRemainingValidationAndCancellationBranches(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CollectContext(context.Background(), root, CollectOptions{}); err == nil {
		t.Fatal("empty options accepted")
	}
	if _, err := CollectContext(context.Background(), root, CollectOptions{Files: true, IgnoredNames: []string{"["}}); err == nil {
		t.Fatal("invalid ignore accepted")
	}
	if _, err := CollectContext(context.Background(), file, CollectOptions{Folders: true}); err == nil {
		t.Fatal("file accepted when excluded")
	}
	entries, err := CollectContext(context.Background(), file, CollectOptions{Files: true, IgnoredNames: []string{"file.*"}})
	if err != nil || len(entries) != 0 {
		t.Fatalf("ignored root file: %+v %v", entries, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ops := scanOperations{abs: filepath.Abs, lstat: os.Lstat, readDir: func(path string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(path)
		cancel()
		return entries, err
	}}
	if _, err := collectContext(ctx, root, CollectOptions{Files: true}, ops); !errors.Is(err, context.Canceled) {
		t.Fatalf("mid-walk cancellation: %v", err)
	}
}

func TestWalkEntryCancellationAndRecursiveFailure(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	var entries []rename.Entry
	ops := scanOperations{readDir: os.ReadDir}
	if err := walk(canceled, root, 0, CollectOptions{Folders: true}, nil, ops, &entries); !errors.Is(err, context.Canceled) {
		t.Fatalf("entry cancellation: %v", err)
	}
	sentinel := errors.New("child read failure")
	ops.readDir = func(path string) ([]os.DirEntry, error) {
		if path == child {
			return nil, sentinel
		}
		return os.ReadDir(path)
	}
	if err := walk(context.Background(), root, 0, CollectOptions{Folders: true}, nil, ops, &entries); !errors.Is(err, sentinel) {
		t.Fatalf("recursive failure: %v", err)
	}
}
