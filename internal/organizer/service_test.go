package organizer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
)

func defaultSpec(t *testing.T) pattern.Spec {
	t.Helper()
	specs, err := pattern.EmbeddedSpecs()
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range specs {
		if spec.Name == "default" {
			return spec
		}
	}
	t.Fatal("default pattern not found")
	return pattern.Spec{}
}

func TestPreviewAndApply(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "my file.TXT")
	if err := os.WriteFile(source, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := New()
	session, err := service.Preview(context.Background(), PreviewRequest{Root: root, IncludeFiles: true, Pattern: defaultSpec(t)})
	if err != nil {
		t.Fatal(err)
	}
	proposals := session.ProposalSnapshot()
	if session.ChangedCount() != 1 || proposals[0].RelativePath != "my file.TXT" {
		t.Fatalf("unexpected preview: %+v", session)
	}
	result, err := service.Apply(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedCount != 1 {
		t.Fatalf("unexpected applied count: %d", result.AppliedCount)
	}
	if _, err := os.Stat(filepath.Join(root, "My File.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRejectsStalePreview(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "my file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := New()
	session, err := service.Preview(context.Background(), PreviewRequest{Root: root, IncludeFiles: true, Pattern: defaultSpec(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "another file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = service.Apply(context.Background(), session)
	if !errors.Is(err, ErrStalePreview) {
		t.Fatalf("expected stale preview, got %v", err)
	}
}

func TestPreviewIgnoresConfiguredTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".DS_Store", "real file.TXT"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	session, err := New().Preview(context.Background(), PreviewRequest{
		Root: root, IncludeFiles: true, Pattern: defaultSpec(t), IgnoredNames: []string{".DS_Store"},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposals := session.ProposalSnapshot()
	if len(proposals) != 1 || proposals[0].OriginalName != "real file.TXT" {
		t.Fatalf("ignored file appeared in preview: %+v", proposals)
	}
}

func TestApplyOneRenamesOnlySelectedProposal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first file.TXT", "second file.TXT"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	service := New()
	session, err := service.Preview(context.Background(), PreviewRequest{Root: root, IncludeFiles: true, Pattern: defaultSpec(t)})
	if err != nil {
		t.Fatal(err)
	}
	selected := ""
	for _, item := range session.ProposalSnapshot() {
		if item.OriginalName == "first file.TXT" {
			selected = item.SourcePath
		}
	}
	result, err := service.ApplyOne(context.Background(), session, selected)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedCount != 1 {
		t.Fatalf("unexpected applied count: %d", result.AppliedCount)
	}
	if _, err := os.Stat(filepath.Join(root, "First File.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "second file.TXT")); err != nil {
		t.Fatal("unselected proposal was renamed")
	}
}

func TestApplyOneRejectsStalePreview(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file name.TXT"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := New()
	session, err := service.Preview(context.Background(), PreviewRequest{Root: root, IncludeFiles: true, Pattern: defaultSpec(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = service.ApplyOne(context.Background(), session, session.ProposalSnapshot()[0].SourcePath)
	if !errors.Is(err, ErrStalePreview) {
		t.Fatalf("expected stale individual preview, got %v", err)
	}
}

func TestPreviewAndApplySingleFilePath(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "single file.TXT")
	if err := os.WriteFile(source, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := New()
	session, err := service.Preview(context.Background(), PreviewRequest{
		Root: source, IncludeFiles: true, IncludeFolders: true, Pattern: defaultSpec(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	proposals := session.ProposalSnapshot()
	if len(proposals) != 1 || proposals[0].RelativePath != "single file.TXT" {
		t.Fatalf("unexpected single-file preview: %+v", proposals)
	}
	if _, err := service.Apply(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Single File.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestPreviewSessionReturnsImmutableSnapshots(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "my file.TXT"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignored := []string{"ignored"}
	spec := defaultSpec(t)
	session, err := New().Preview(context.Background(), PreviewRequest{
		Root: root, IncludeFiles: true, IgnoredNames: ignored, Pattern: spec,
	})
	if err != nil {
		t.Fatal(err)
	}
	first := session.ProposalSnapshot()
	first[0].ProposedName = "mutated"
	ignored[0] = "mutated"
	if got := session.ProposalSnapshot()[0].ProposedName; got == "mutated" {
		t.Fatal("proposal snapshot mutated the sealed session")
	}
	if got := session.request.IgnoredNames[0]; got != "ignored" {
		t.Fatalf("ignored names mutated the sealed session: %q", got)
	}
}
