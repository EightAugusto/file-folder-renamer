package apply

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

func TestDetectCollisionSameDirectory(t *testing.T) {
	rootDirectory := t.TempDir()
	proposals := []rename.Proposal{
		{SourcePath: filepath.Join(rootDirectory, "a.txt"), OriginalName: "a.txt", ProposedName: "same.txt", ParentDir: rootDirectory, Kind: rename.NodeKindFile, Changed: true},
		{SourcePath: filepath.Join(rootDirectory, "b.txt"), OriginalName: "b.txt", ProposedName: "same.txt", ParentDir: rootDirectory, Kind: rename.NodeKindFile, Changed: true},
	}

	validationErr := Validate(proposals)
	if validationErr == nil {
		t.Fatal("expected collision error")
	}
}

func TestValidateRejectsUnsafeProposals(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	base := rename.Proposal{SourcePath: source, OriginalName: "source.txt", ProposedName: "target.txt", ParentDir: root, Kind: rename.NodeKindFile, Changed: true}
	tests := []struct {
		name   string
		mutate func(*rename.Proposal)
	}{
		{name: "relative source", mutate: func(item *rename.Proposal) { item.SourcePath = "source.txt" }},
		{name: "wrong parent", mutate: func(item *rename.Proposal) { item.ParentDir = filepath.Dir(root) }},
		{name: "unknown kind", mutate: func(item *rename.Proposal) { item.Kind = "device" }},
		{name: "empty target", mutate: func(item *rename.Proposal) { item.ProposedName = "" }},
		{name: "dot target", mutate: func(item *rename.Proposal) { item.ProposedName = "." }},
		{name: "parent target", mutate: func(item *rename.Proposal) { item.ProposedName = ".." }},
		{name: "nested target", mutate: func(item *rename.Proposal) { item.ProposedName = "folder/file.txt" }},
		{name: "windows separator", mutate: func(item *rename.Proposal) { item.ProposedName = `folder\file.txt` }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			item := base
			testCase.mutate(&item)
			if err := Validate([]rename.Proposal{item}); err == nil {
				t.Fatal("expected unsafe proposal to be rejected")
			}
		})
	}
	if err := Validate([]rename.Proposal{base, base}); err == nil {
		t.Fatal("expected duplicate source to be rejected")
	}
}

func TestRejectsOccupiedUnchangedTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateFilesystem([]rename.Proposal{{SourcePath: source, OriginalName: "source.txt", ProposedName: "target.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true}})
	if err == nil {
		t.Fatal("expected occupied target to be rejected")
	}
}

func TestApplyStagesRenameChainWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	proposals := []rename.Proposal{
		{SourcePath: a, OriginalName: "a.txt", ProposedName: "b.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true},
		{SourcePath: b, OriginalName: "b.txt", ProposedName: "c.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true},
	}
	if err := Apply(proposals); err != nil {
		t.Fatal(err)
	}
	dataB, _ := os.ReadFile(filepath.Join(root, "b.txt"))
	dataC, _ := os.ReadFile(filepath.Join(root, "c.txt"))
	if string(dataB) != "a" || string(dataC) != "b" {
		t.Fatalf("chain contents were not preserved: b=%q c=%q", dataB, dataC)
	}
}

func TestApplyStagesSwapAndCaseOnlyRename(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "alpha.txt")
	b := filepath.Join(root, "beta.txt")
	if err := os.WriteFile(a, []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Apply([]rename.Proposal{
		{SourcePath: a, OriginalName: "alpha.txt", ProposedName: "beta.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true},
		{SourcePath: b, OriginalName: "beta.txt", ProposedName: "alpha.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true},
	}); err != nil {
		t.Fatal(err)
	}
	dataA, _ := os.ReadFile(a)
	dataB, _ := os.ReadFile(b)
	if string(dataA) != "beta" || string(dataB) != "alpha" {
		t.Fatalf("swap failed: alpha=%q beta=%q", dataA, dataB)
	}

	caseSource := filepath.Join(root, "mixed.TXT")
	if err := os.WriteFile(caseSource, []byte("case"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Apply([]rename.Proposal{{SourcePath: caseSource, OriginalName: "mixed.TXT", ProposedName: "mixed.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "mixed.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRollsBackOnFinalizeFailure(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	proposals := []rename.Proposal{
		{SourcePath: a, OriginalName: "a.txt", ProposedName: "c.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true},
		{SourcePath: b, OriginalName: "b.txt", ProposedName: "d.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true},
	}
	renameCalls := 0
	err := applyWithOps(proposals, fileOps{lstat: os.Lstat, rename: func(from, to string) error {
		renameCalls++
		if renameCalls == 4 {
			return errors.New("injected failure")
		}
		return os.Rename(from, to)
	}})
	var applyErr *ApplyError
	if !errors.As(err, &applyErr) || len(applyErr.RollbackErrors) != 0 {
		t.Fatalf("expected successful rollback, got %v", err)
	}
	if data, readErr := os.ReadFile(a); readErr != nil || string(data) != "a" {
		t.Fatalf("a was not restored: %q %v", data, readErr)
	}
	if data, readErr := os.ReadFile(b); readErr != nil || string(data) != "b" {
		t.Fatalf("b was not restored: %q %v", data, readErr)
	}
}

func TestApplyReportsRollbackFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	if err := os.WriteFile(source, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := applyWithOps([]rename.Proposal{{SourcePath: source, OriginalName: "source.txt", ProposedName: "target.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true}}, fileOps{
		lstat: os.Lstat,
		rename: func(from, to string) error {
			calls++
			if calls >= 2 {
				return errors.New("injected failure")
			}
			return os.Rename(from, to)
		},
	})
	var applyErr *ApplyError
	if !errors.As(err, &applyErr) || len(applyErr.RollbackErrors) != 1 {
		t.Fatalf("expected rollback failure details, got %v", err)
	}
}

func TestApplyNestedFolderAfterChild(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent folder")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "child file.txt")
	if err := os.WriteFile(child, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	proposals := []rename.Proposal{
		{SourcePath: child, OriginalName: "child file.txt", ProposedName: "Child File.txt", ParentDir: parent, Kind: rename.NodeKindFile, Depth: 2, Changed: true},
		{SourcePath: parent, OriginalName: "parent folder", ProposedName: "Parent Folder", ParentDir: root, Kind: rename.NodeKindFolder, Depth: 1, Changed: true},
	}
	if err := Apply(proposals); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Parent Folder", "Child File.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRollsBackEveryRenameFailureStage(t *testing.T) {
	for _, failAt := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprintf("rename_%d", failAt), func(t *testing.T) {
			root := t.TempDir()
			a := filepath.Join(root, "a.txt")
			b := filepath.Join(root, "b.txt")
			if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
				t.Fatal(err)
			}
			calls := 0
			err := applyWithOps([]rename.Proposal{
				{SourcePath: a, OriginalName: "a.txt", ProposedName: "c.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true},
				{SourcePath: b, OriginalName: "b.txt", ProposedName: "d.txt", ParentDir: root, Kind: rename.NodeKindFile, Depth: 1, Changed: true},
			}, fileOps{lstat: os.Lstat, rename: func(from, to string) error {
				calls++
				if calls == failAt {
					return errors.New("injected rename failure")
				}
				return os.Rename(from, to)
			}})
			var applyErr *ApplyError
			if !errors.As(err, &applyErr) {
				t.Fatalf("expected ApplyError, got %v", err)
			}
			if data, readErr := os.ReadFile(a); readErr != nil || string(data) != "a" {
				t.Fatalf("a was not restored: %q %v", data, readErr)
			}
			if data, readErr := os.ReadFile(b); readErr != nil || string(data) != "b" {
				t.Fatalf("b was not restored: %q %v", data, readErr)
			}
		})
	}
}

func TestTemporaryNameCollisionsAreDeterministic(t *testing.T) {
	root := t.TempDir()
	occupied := filepath.Join(root, ".occupied")
	if err := os.WriteFile(occupied, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err := unusedTempPath(root, fileOps{
		lstat: os.Lstat,
		randomName: func() (string, error) {
			calls++
			return ".occupied", nil
		},
	})
	if err == nil || calls != 32 {
		t.Fatalf("expected 32 deterministic collisions, calls=%d err=%v", calls, err)
	}
}
