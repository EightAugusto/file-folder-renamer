package rename

import (
	"path/filepath"
)

type BuildOptions struct {
	Pipeline *Pipeline
	Files    bool
	Folders  bool
}

func Build(entries []Entry, opts BuildOptions) ([]Proposal, error) {
	proposals := make([]Proposal, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind == NodeKindFile && !opts.Files {
			continue
		}
		if entry.Kind == NodeKindFolder && !opts.Folders {
			continue
		}
		if entry.Kind == NodeKindFolder && entry.Depth == 0 {
			continue
		}

		original := entry.Name
		proposed := original
		nameParts := Parse(original)
		if entry.Kind == NodeKindFile {
			if opts.Pipeline != nil && !nameParts.Protected {
				transformedStem, err := opts.Pipeline.Apply(nameParts.Stem)
				if err != nil {
					return nil, err
				}
				proposed = transformedStem + NormalizeExtension(nameParts.Extension)
			} else {
				proposed = nameParts.Stem + NormalizeExtension(nameParts.Extension)
			}
		} else if opts.Pipeline != nil && !nameParts.Protected {
			transformedName, err := opts.Pipeline.Apply(original)
			if err != nil {
				return nil, err
			}
			proposed = transformedName
		}

		proposals = append(proposals, Proposal{
			SourcePath:    entry.SourcePath,
			OriginalName:  original,
			ProposedName:  proposed,
			Kind:          entry.Kind,
			Depth:         entry.Depth,
			Changed:       original != proposed,
			RuleChainName: pipelineName(opts.Pipeline),
			ParentDir:     filepath.Dir(entry.SourcePath),
		})
	}
	return proposals, nil
}

func pipelineName(renamePipeline *Pipeline) string {
	if renamePipeline == nil {
		return ""
	}
	return renamePipeline.Name()
}
