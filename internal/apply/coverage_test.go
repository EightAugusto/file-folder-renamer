package apply

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

func TestApplyErrorFormattingAndUnwrap(t *testing.T) {
	sentinel := errors.New("cause")
	for _, rollback := range [][]error{nil, {errors.New("rollback")}} {
		err := &ApplyError{Cause: sentinel, RollbackErrors: rollback}
		if err.Error() == "" || !errors.Is(err, sentinel) {
			t.Fatalf("unexpected apply error: %v", err)
		}
	}
}

func TestValidateFilesystemInjectedBranches(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	if err := os.WriteFile(source, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	proposal := rename.Proposal{SourcePath: source, ParentDir: root, ProposedName: "target", Kind: rename.NodeKindFile, Changed: true}
	sentinel := errors.New("inspect failure")
	if err := validateFilesystem([]rename.Proposal{proposal}, fileOps{lstat: func(path string) (os.FileInfo, error) {
		if path == target {
			return nil, sentinel
		}
		return os.Lstat(path)
	}}); !errors.Is(err, sentinel) {
		t.Fatalf("target inspection: %v", err)
	}
	if err := validateFilesystem([]rename.Proposal{proposal}, fileOps{lstat: func(string) (os.FileInfo, error) { return nil, sentinel }}); !errors.Is(err, sentinel) {
		t.Fatalf("source inspection: %v", err)
	}
	if err := validateFilesystem([]rename.Proposal{{Changed: true}}, fileOps{}); err == nil {
		t.Fatal("invalid proposal accepted")
	}
}

func TestApplyWithOpsInjectedPreFinalizeFailures(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	proposal := rename.Proposal{SourcePath: source, ParentDir: root, ProposedName: "target", Kind: rename.NodeKindFile, Changed: true}
	sentinel := errors.New("injected")
	if err := applyWithOps([]rename.Proposal{proposal}, fileOps{lstat: os.Lstat, rename: os.Rename, randomName: func() (string, error) { return "", sentinel }}); !errors.Is(err, sentinel) {
		t.Fatalf("temporary name: %v", err)
	}
	if err := applyWithOps([]rename.Proposal{proposal}, fileOps{lstat: os.Lstat, rename: func(string, string) error { return sentinel }}); !errors.Is(err, sentinel) {
		t.Fatalf("stage: %v", err)
	}
	if err := applyWithOps(nil, fileOps{}); err != nil {
		t.Fatalf("empty apply with defaults: %v", err)
	}
	if err := applyWithOps([]rename.Proposal{{Changed: true}}, fileOps{}); err == nil {
		t.Fatal("invalid proposal accepted")
	}
}

func TestApplyDetectsTargetChangeAfterStaging(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		targetInfo func() (os.FileInfo, error)
	}{
		{name: "occupied", targetInfo: func() (os.FileInfo, error) { return fakeInfo{}, nil }},
		{name: "inspect error", targetInfo: func() (os.FileInfo, error) { return nil, errors.New("inspect failure") }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "target")
			if err := os.WriteFile(source, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			targetChecks := 0
			lstat := func(path string) (os.FileInfo, error) {
				if path == target {
					targetChecks++
					if targetChecks > 1 {
						return testCase.targetInfo()
					}
				}
				return os.Lstat(path)
			}
			proposal := rename.Proposal{SourcePath: source, ParentDir: root, ProposedName: "target", Kind: rename.NodeKindFile, Changed: true}
			err := applyWithOps([]rename.Proposal{proposal}, fileOps{lstat: lstat, rename: os.Rename, randomName: func() (string, error) { return ".temp", nil }})
			if err == nil {
				t.Fatal("target change accepted")
			}
			if _, statErr := os.Stat(source); statErr != nil {
				t.Fatalf("source not rolled back: %v", statErr)
			}
		})
	}
}

func TestTemporaryNameErrors(t *testing.T) {
	sentinel := errors.New("random failure")
	if _, err := randomTemporaryNameFrom(errorReader{sentinel}); !errors.Is(err, sentinel) {
		t.Fatalf("random failure: %v", err)
	}
	if _, err := unusedTempPath(t.TempDir(), fileOps{lstat: os.Lstat, randomName: func() (string, error) { return "nested/name", nil }}); err == nil {
		t.Fatal("invalid temporary name accepted")
	}
	if _, err := unusedTempPath(t.TempDir(), fileOps{lstat: func(string) (os.FileInfo, error) { return nil, sentinel }, randomName: func() (string, error) { return ".temp", nil }}); !errors.Is(err, sentinel) {
		t.Fatalf("temporary inspection: %v", err)
	}
}

func TestChangedProposalsSortsDifferentParents(t *testing.T) {
	items := changedProposals([]rename.Proposal{
		{SourcePath: "/z/item", ParentDir: "/z", Depth: 1, Changed: true},
		{SourcePath: "/a/item", ParentDir: "/a", Depth: 1, Changed: true},
		{Changed: false},
	})
	if len(items) != 2 || items[0].ParentDir != "/a" {
		t.Fatalf("unexpected ordering: %+v", items)
	}
}

func TestRollbackFailureBranches(t *testing.T) {
	sentinel := errors.New("failure")
	journal := []renameOperation{{from: "/from", to: "/to"}}
	err := rollbackFailure(sentinel, journal, fileOps{
		lstat:  func(string) (os.FileInfo, error) { return fakeInfo{}, nil },
		rename: func(string, string) error { t.Fatal("rename called for occupied destination"); return nil },
	})
	applyErr := err.(*ApplyError)
	if len(applyErr.RollbackErrors) != 1 || !strings.Contains(applyErr.RollbackErrors[0].Error(), "destination became occupied") {
		t.Fatalf("occupied rollback: %v", err)
	}
	err = rollbackFailure(sentinel, journal, fileOps{
		lstat:  func(string) (os.FileInfo, error) { return nil, errors.New("inspect") },
		rename: func(string, string) error { t.Fatal("rename called after inspect failure"); return nil },
	})
	applyErr = err.(*ApplyError)
	if len(applyErr.RollbackErrors) != 1 || !strings.Contains(applyErr.RollbackErrors[0].Error(), "inspect rollback") {
		t.Fatalf("inspect rollback: %v", err)
	}
}

type errorReader struct{ err error }

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

type fakeInfo struct{}

func (fakeInfo) Name() string       { return "fake" }
func (fakeInfo) Size() int64        { return 0 }
func (fakeInfo) Mode() os.FileMode  { return 0 }
func (fakeInfo) ModTime() time.Time { return time.Time{} }
func (fakeInfo) IsDir() bool        { return false }
func (fakeInfo) Sys() any           { return nil }
