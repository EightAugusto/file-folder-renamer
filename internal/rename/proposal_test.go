package rename

import (
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func TestBuildReturnsProposalData(t *testing.T) {
	entries := []Entry{
		{SourcePath: "/tmp/A.txt", Name: "A.txt", Kind: NodeKindFile},
	}
	proposals, err := Build(entries, BuildOptions{
		Pipeline: New("custom", []rules.Rule{
			rules.CaseRule{Mode: rules.CaseModeLower},
			rules.ReplaceRunesRule{Remove: map[rune]struct{}{' ': {}}, Replacement: "-"},
		}),
		Files:   true,
		Folders: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(proposals) != 1 || !proposals[0].Changed {
		t.Fatalf("expected one changed proposal")
	}
}

func TestBuildPreservesFileExtension(t *testing.T) {
	entries := []Entry{
		{SourcePath: "/tmp/my file.TXT", Name: "my file.TXT", Kind: NodeKindFile},
	}
	proposals, err := Build(entries, BuildOptions{
		Pipeline: New("custom", []rules.Rule{
			rules.CaseRule{Mode: rules.CaseModeLower},
			rules.ReplaceRunesRule{Remove: map[rune]struct{}{' ': {}}, Replacement: "-"},
		}),
		Files:   true,
		Folders: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposals[0].ProposedName != "my-file.txt" {
		t.Fatalf("got %q", proposals[0].ProposedName)
	}
}

func TestBuildPreservesCompoundExtension(t *testing.T) {
	entries := []Entry{
		{SourcePath: "/tmp/backup file.TAR.GZ", Name: "backup file.TAR.GZ", Kind: NodeKindFile},
	}
	proposals, err := Build(entries, BuildOptions{
		Pipeline: New("custom", []rules.Rule{
			rules.CaseRule{Mode: rules.CaseModeLower},
			rules.ReplaceRunesRule{Remove: map[rune]struct{}{' ': {}}, Replacement: "-"},
		}),
		Files: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposals[0].ProposedName != "backup-file.tar.gz" {
		t.Fatalf("got %q", proposals[0].ProposedName)
	}
}

func TestBuildPreservesHiddenNames(t *testing.T) {
	entries := []Entry{
		{SourcePath: "/tmp/.gitignore", Name: ".gitignore", Kind: NodeKindFile, Depth: 1},
		{SourcePath: "/tmp/.GITIGNORE", Name: ".GITIGNORE", Kind: NodeKindFile, Depth: 1},
		{SourcePath: "/tmp/.config", Name: ".config", Kind: NodeKindFolder, Depth: 1},
	}
	proposals, err := Build(entries, BuildOptions{
		Pipeline: New("custom", []rules.Rule{
			rules.CaseRule{Mode: rules.CaseModeUpper},
		}),
		Files:   true,
		Folders: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposals[0].ProposedName != ".gitignore" {
		t.Fatalf("got %q", proposals[0].ProposedName)
	}
	if proposals[1].ProposedName != ".gitignore" {
		t.Fatalf("got %q", proposals[1].ProposedName)
	}
	if proposals[2].ProposedName != ".config" {
		t.Fatalf("got %q", proposals[2].ProposedName)
	}
}
