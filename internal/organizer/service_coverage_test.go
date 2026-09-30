package organizer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/scanner"
)

func TestPreviewValidationAndInjectedFailures(t *testing.T) {
	sentinel := errors.New("injected")
	valid := PreviewRequest{Root: t.TempDir(), IncludeFiles: true, Pattern: defaultSpec(t)}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Preview(canceled, valid); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	for _, request := range []PreviewRequest{{IncludeFiles: true}, {Root: valid.Root}, {Root: valid.Root, IncludeFiles: true}} {
		if _, err := New().Preview(context.Background(), request); err == nil {
			t.Fatalf("expected request to fail: %+v", request)
		}
	}

	tests := []struct {
		name   string
		mutate func(*Service)
	}{
		{name: "absolute", mutate: func(service *Service) { service.abs = func(string) (string, error) { return "", sentinel } }},
		{name: "scan", mutate: func(service *Service) {
			service.scan = func(context.Context, string, scanner.CollectOptions) ([]rename.Entry, error) { return nil, sentinel }
		}},
		{name: "plan", mutate: func(service *Service) {
			service.plan = func([]rename.Entry, rename.BuildOptions) ([]rename.Proposal, error) { return nil, sentinel }
		}},
		{name: "relative", mutate: func(service *Service) {
			service.plan = func([]rename.Entry, rename.BuildOptions) ([]rename.Proposal, error) {
				return []rename.Proposal{{SourcePath: filepath.Join(valid.Root, "file"), Kind: rename.NodeKindFile}}, nil
			}
			service.relative = func(string, string) (string, error) { return "", sentinel }
		}},
		{name: "validate", mutate: func(service *Service) { service.validate = func([]rename.Proposal) error { return sentinel } }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service := New()
			testCase.mutate(service)
			if _, err := service.Preview(context.Background(), valid); !errors.Is(err, sentinel) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestSessionAccessorsAndNilSnapshot(t *testing.T) {
	if (*PreviewSession)(nil).ProposalSnapshot() != nil {
		t.Fatal("nil session returned proposals")
	}
	proposal := rename.Proposal{Changed: true}
	session := NewPreviewSession("/root", "pattern", []rename.Proposal{proposal})
	if session.Root() != "/root" || session.PatternName() != "pattern" || session.UnchangedCount() != 0 {
		t.Fatalf("unexpected session accessors: %+v", session)
	}
	if session.ChangedCount() != 1 {
		t.Fatalf("unexpected constructed metadata: %+v", session)
	}
	_ = proposalFingerprint(nil)
}

func TestApplyGuardAndInjectedFailures(t *testing.T) {
	if _, err := New().Apply(context.Background(), nil); err == nil {
		t.Fatal("nil batch session accepted")
	}
	if _, err := New().ApplyOne(context.Background(), nil, ""); err == nil {
		t.Fatal("nil single session accepted")
	}
	root := t.TempDir()
	source := filepath.Join(root, "my file.TXT")
	if err := os.WriteFile(source, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	service := New()
	session, err := service.Preview(context.Background(), PreviewRequest{Root: root, IncludeFiles: true, IncludeFolders: true, IgnoredNames: []string{"ignored"}, Pattern: defaultSpec(t)})
	if err != nil {
		t.Fatal(err)
	}
	if !session.request.IncludeFiles || !session.request.IncludeFolders || len(session.request.IgnoredNames) != 1 || session.UnchangedCount() != 0 || session.Root() != root || session.PatternName() == "" {
		t.Fatalf("preview metadata mismatch: %+v", session)
	}
	if _, err := service.ApplyOne(context.Background(), session, "missing"); err == nil {
		t.Fatal("missing selection accepted")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Apply(canceled, session); !errors.Is(err, context.Canceled) {
		t.Fatalf("batch cancellation: %v", err)
	}
	if _, err := service.ApplyOne(canceled, session, source); !errors.Is(err, context.Canceled) {
		t.Fatalf("single cancellation: %v", err)
	}

	sentinel := errors.New("apply failure")
	service.apply = func([]rename.Proposal) error { return sentinel }
	if _, err := service.Apply(context.Background(), session); !errors.Is(err, sentinel) {
		t.Fatalf("batch apply failure: %v", err)
	}
	if _, err := service.ApplyOne(context.Background(), session, source); !errors.Is(err, sentinel) {
		t.Fatalf("single apply failure: %v", err)
	}
	service.scan = func(context.Context, string, scanner.CollectOptions) ([]rename.Entry, error) { return nil, sentinel }
	if _, err := service.Apply(context.Background(), session); !errors.Is(err, ErrStalePreview) || !errors.Is(err, sentinel) {
		t.Fatalf("batch refresh failure: %v", err)
	}
	if _, err := service.ApplyOne(context.Background(), session, source); !errors.Is(err, ErrStalePreview) || !errors.Is(err, sentinel) {
		t.Fatalf("single refresh failure: %v", err)
	}
}

func TestApplyOneRejectsUnchangedAndMissingFreshSelection(t *testing.T) {
	root := t.TempDir()
	unchanged := filepath.Join(root, "Already.txt")
	if err := os.WriteFile(unchanged, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	service := New()
	session, err := service.Preview(context.Background(), PreviewRequest{Root: root, IncludeFiles: true, Pattern: defaultSpec(t)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyOne(context.Background(), session, unchanged); err == nil {
		t.Fatal("unchanged proposal accepted")
	}

	changed := filepath.Join(root, "my file.TXT")
	if err := os.WriteFile(changed, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	session, err = service.Preview(context.Background(), PreviewRequest{Root: root, IncludeFiles: true, Pattern: defaultSpec(t)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(changed); err != nil {
		t.Fatal(err)
	}
	fresh, err := service.Preview(context.Background(), session.request)
	if err != nil {
		t.Fatal(err)
	}
	session.fingerprint = fresh.fingerprint
	if _, err := service.ApplyOne(context.Background(), session, changed); !errors.Is(err, ErrStalePreview) {
		t.Fatalf("missing fresh selection: %v", err)
	}
}
