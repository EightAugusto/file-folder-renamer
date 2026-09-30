package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CollectContext(ctx, t.TempDir(), CollectOptions{Files: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestCollectReturnsRootFilesAndFolders(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a.txt"), "x")
	mustMkdir(t, filepath.Join(root, "sub"))

	got, err := CollectContext(context.Background(), root, CollectOptions{Files: true, Folders: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected root + file + folder, got %d", len(got))
	}
}

func TestCollectSkipsIgnoredFilesAndFolderTrees(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".DS_Store"), "metadata")
	mustWriteFile(t, filepath.Join(root, "document.txt"), "real")
	ignoredFolder := filepath.Join(root, ".Trash-1000")
	mustMkdir(t, ignoredFolder)
	mustWriteFile(t, filepath.Join(ignoredFolder, "deleted.txt"), "trash")

	got, err := CollectContext(context.Background(), root, CollectOptions{
		Files: true, Folders: true, IgnoredNames: []string{".DS_Store", ".Trash-*"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Name != "document.txt" {
		t.Fatalf("ignored entries were collected: %+v", got)
	}
}

func TestCollectAcceptsSingleFileRoot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "single file.txt")
	mustWriteFile(t, file, "content")
	got, err := CollectContext(context.Background(), file, CollectOptions{Files: true, Folders: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SourcePath != file || got[0].Kind != "file" || got[0].Depth != 0 {
		t.Fatalf("unexpected single-file collection: %+v", got)
	}
}

func TestCollectCanIgnoreSingleFileRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), ".DS_Store")
	mustWriteFile(t, file, "metadata")
	got, err := CollectContext(context.Background(), file, CollectOptions{Files: true, IgnoredNames: []string{".DS_Store"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("ignored selected file was collected: %+v", got)
	}
}

func TestCollectIsSortedAndDoesNotFollowSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	mustMkdir(t, target)
	mustWriteFile(t, filepath.Join(target, "inside.txt"), "inside")
	mustWriteFile(t, filepath.Join(root, "z.txt"), "z")
	link := filepath.Join(root, "a-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	entries, err := CollectContext(context.Background(), root, CollectOptions{Files: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].Name != "a-link" || entries[1].Name != "inside.txt" || entries[2].Name != "z.txt" {
		t.Fatalf("unexpected deterministic scan: %+v", entries)
	}
	for _, entry := range entries {
		if entry.SourcePath == filepath.Join(link, "inside.txt") {
			t.Fatal("scanner followed a directory symlink")
		}
	}
}

func TestCollectTreatsRootSymlinkAsFile(t *testing.T) {
	target := t.TempDir()
	mustWriteFile(t, filepath.Join(target, "inside.txt"), "inside")
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	entries, err := CollectContext(context.Background(), link, CollectOptions{Files: true, Folders: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].SourcePath != link || entries[0].Kind != "file" || entries[0].Depth != 0 {
		t.Fatalf("root symlink was followed: %+v", entries)
	}
	if _, err := CollectContext(context.Background(), link, CollectOptions{Folders: true}); err == nil {
		t.Fatal("root symlink accepted when files are excluded")
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
