package rename

import (
	"errors"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func TestPipelineNilAndFailureBranches(t *testing.T) {
	var pipeline *Pipeline
	if pipeline.Name() != "" {
		t.Fatal("nil pipeline has a name")
	}
	if got, err := pipeline.Apply("same"); err != nil || got != "same" {
		t.Fatalf("nil pipeline: %q %v", got, err)
	}
	sentinel := errors.New("rule failure")
	pipeline = New("failure", []rules.Rule{errorRule{sentinel}})
	if pipeline.Name() != "failure" {
		t.Fatal("pipeline name mismatch")
	}
	if _, err := pipeline.Apply("input"); !errors.Is(err, sentinel) {
		t.Fatalf("rule failure: %v", err)
	}
}

func TestBuildCoversKindsFiltersAndFailures(t *testing.T) {
	entries := []Entry{
		{SourcePath: "/tmp/file.txt", Name: "file.txt", Kind: NodeKindFile},
		{SourcePath: "/tmp/folder", Name: "folder", Kind: NodeKindFolder, Depth: 1},
		{SourcePath: "/tmp", Name: "tmp", Kind: NodeKindFolder, Depth: 0},
	}
	proposals, err := Build(entries, BuildOptions{})
	if err != nil || len(proposals) != 0 {
		t.Fatalf("disabled filters: %+v %v", proposals, err)
	}
	proposals, err = Build(entries, BuildOptions{Files: true, Folders: true})
	if err != nil || len(proposals) != 2 || proposals[0].RuleChainName != "" || proposals[1].ProposedName != "folder" {
		t.Fatalf("nil pipeline proposals: %+v %v", proposals, err)
	}
	sentinel := errors.New("rule failure")
	failing := New("failure", []rules.Rule{errorRule{sentinel}})
	for _, entry := range entries[:2] {
		_, err := Build([]Entry{entry}, BuildOptions{Pipeline: failing, Files: true, Folders: true})
		if !errors.Is(err, sentinel) {
			t.Fatalf("%s failure: %v", entry.Kind, err)
		}
	}
	folderPipeline := New("folder", []rules.Rule{rules.CaseRule{Mode: rules.CaseModeUpper}})
	proposals, err = Build([]Entry{{SourcePath: "/tmp/folder", Name: "folder", Kind: NodeKindFolder, Depth: 1}}, BuildOptions{Pipeline: folderPipeline, Folders: true})
	if err != nil || len(proposals) != 1 || proposals[0].ProposedName != "FOLDER" {
		t.Fatalf("folder transformation: %+v %v", proposals, err)
	}
}

type errorRule struct{ err error }

func (rule errorRule) Apply(string) (string, error) { return "", rule.err }
func (errorRule) Kind() rules.Kind                  { return rules.KindCase }
