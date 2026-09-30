package organizer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/eightaugusto/file-folder-renamer/internal/apply"
	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/scanner"
)

var ErrStalePreview = errors.New("preview is stale; scan the folder again")

type PreviewRequest struct {
	Root           string
	IncludeFiles   bool
	IncludeFolders bool
	IgnoredNames   []string
	Pattern        pattern.Spec
}

type PreviewSession struct {
	root           string
	patternName    string
	changedCount   int
	unchangedCount int

	request     PreviewRequest
	fingerprint [sha256.Size]byte
	proposals   []rename.Proposal
}

func (session *PreviewSession) Root() string        { return session.root }
func (session *PreviewSession) PatternName() string { return session.patternName }
func (session *PreviewSession) ChangedCount() int   { return session.changedCount }
func (session *PreviewSession) UnchangedCount() int { return session.unchangedCount }

// ProposalSnapshot returns a copy of the proposals sealed into this preview.
func (session *PreviewSession) ProposalSnapshot() []rename.Proposal {
	if session == nil {
		return nil
	}
	return append([]rename.Proposal(nil), session.proposals...)
}

// NewPreviewSession builds an immutable session snapshot. It is primarily
// useful to adapters that need to restore already-computed presentation state.
func NewPreviewSession(root, patternName string, proposals []rename.Proposal) *PreviewSession {
	copied := append([]rename.Proposal(nil), proposals...)
	changed := 0
	for _, proposal := range copied {
		if proposal.Changed {
			changed++
		}
	}
	return &PreviewSession{
		root: root, patternName: patternName, proposals: copied,
		changedCount: changed, unchangedCount: len(copied) - changed,
	}
}

type ApplyResult struct {
	AppliedCount int
	CompletedAt  time.Time
}

type Service struct {
	scan     func(context.Context, string, scanner.CollectOptions) ([]rename.Entry, error)
	plan     func([]rename.Entry, rename.BuildOptions) ([]rename.Proposal, error)
	validate func([]rename.Proposal) error
	apply    func([]rename.Proposal) error
	abs      func(string) (string, error)
	relative func(string, string) (string, error)
	now      func() time.Time
}

func New() *Service {
	return &Service{
		scan: scanner.CollectContext, plan: rename.Build, validate: apply.ValidateFilesystem, apply: apply.Apply,
		abs: filepath.Abs, relative: filepath.Rel,
		now: time.Now,
	}
}

func (service *Service) Preview(ctx context.Context, request PreviewRequest) (*PreviewSession, error) {
	// Validate and copy the request before sealing it for apply-time rechecks.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.Root == "" {
		return nil, errors.New("choose a folder to organize")
	}
	if !request.IncludeFiles && !request.IncludeFolders {
		return nil, errors.New("include files, folders, or both")
	}
	root, err := service.abs(request.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root folder: %w", err)
	}
	request.Root = root
	request.IgnoredNames = append([]string(nil), request.IgnoredNames...)
	request.Pattern = request.Pattern.Clone()
	definition, err := pattern.Build(request.Pattern)
	if err != nil {
		return nil, err
	}
	renamePipeline := rename.New(definition.Name, definition.Rules)
	// Build and validate the exact proposal set shown by the UI.
	entries, err := service.scan(ctx, root, scanner.CollectOptions{
		Files: request.IncludeFiles, Folders: request.IncludeFolders, IgnoredNames: request.IgnoredNames,
	})
	if err != nil {
		return nil, err
	}
	proposals, err := service.plan(entries, rename.BuildOptions{
		Pipeline: renamePipeline,
		Files:    request.IncludeFiles,
		Folders:  request.IncludeFolders,
	})
	if err != nil {
		return nil, err
	}
	changedCount := 0
	for index := range proposals {
		relativePath, relativeErr := service.relative(root, proposals[index].SourcePath)
		if relativeErr != nil {
			return nil, fmt.Errorf("make proposal path %q relative to %q: %w", proposals[index].SourcePath, root, relativeErr)
		}
		if relativePath == "." && proposals[index].Kind == rename.NodeKindFile {
			relativePath = filepath.Base(proposals[index].SourcePath)
		}
		proposals[index].RelativePath = relativePath
		if proposals[index].Changed {
			changedCount++
		}
	}
	if err := service.validate(proposals); err != nil {
		return nil, err
	}
	fingerprint := proposalFingerprint(proposals)
	return &PreviewSession{
		root:           root,
		patternName:    definition.Name,
		changedCount:   changedCount,
		unchangedCount: len(proposals) - changedCount,
		request:        request,
		fingerprint:    fingerprint,
		proposals:      append([]rename.Proposal(nil), proposals...),
	}, nil
}

func (service *Service) ApplyOne(ctx context.Context, session *PreviewSession, sourcePath string) (ApplyResult, error) {
	if session == nil {
		return ApplyResult{}, errors.New("preview is required before applying a change")
	}
	selected := -1
	for index, proposal := range session.proposals {
		if proposal.SourcePath == sourcePath {
			selected = index
			break
		}
	}
	if selected < 0 {
		return ApplyResult{}, errors.New("selected preview item is unavailable")
	}
	if !session.proposals[selected].Changed {
		return ApplyResult{}, errors.New("selected preview item has no change to apply")
	}
	if err := ctx.Err(); err != nil {
		return ApplyResult{}, err
	}
	// A single-row action still checks the complete preview for changes.
	fresh, err := service.Preview(ctx, session.request)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("%w: %w", ErrStalePreview, err)
	}
	if fresh.fingerprint != session.fingerprint {
		return ApplyResult{}, ErrStalePreview
	}
	freshSelected := -1
	for index, proposal := range fresh.proposals {
		if proposal.SourcePath == sourcePath {
			freshSelected = index
			break
		}
	}
	if freshSelected < 0 {
		return ApplyResult{}, ErrStalePreview
	}
	if err := service.apply([]rename.Proposal{fresh.proposals[freshSelected]}); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{AppliedCount: 1, CompletedAt: service.now()}, nil
}

func (service *Service) Apply(ctx context.Context, session *PreviewSession) (ApplyResult, error) {
	if session == nil {
		return ApplyResult{}, errors.New("preview is required before applying changes")
	}
	if err := ctx.Err(); err != nil {
		return ApplyResult{}, err
	}
	// Rebuild from the sealed request; the displayed snapshot cannot authorize
	// a rename after its source tree or proposal set has changed.
	fresh, err := service.Preview(ctx, session.request)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("%w: %w", ErrStalePreview, err)
	}
	if fresh.fingerprint != session.fingerprint {
		return ApplyResult{}, ErrStalePreview
	}
	if err := service.apply(fresh.proposals); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{AppliedCount: fresh.changedCount, CompletedAt: service.now()}, nil
}

func proposalFingerprint(proposals []rename.Proposal) [sha256.Size]byte {
	type signature struct {
		SourcePath   string
		OriginalName string
		ProposedName string
		Kind         rename.NodeKind
		Depth        int
		ParentDir    string
	}
	values := make([]signature, len(proposals))
	for index, item := range proposals {
		values[index] = signature{
			SourcePath: item.SourcePath, OriginalName: item.OriginalName, ProposedName: item.ProposedName,
			Kind: item.Kind, Depth: item.Depth, ParentDir: item.ParentDir,
		}
	}
	encoded, _ := json.Marshal(values) // signature contains only JSON-safe scalar values
	return sha256.Sum256(encoded)
}
