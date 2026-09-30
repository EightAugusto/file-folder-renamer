package rename

type NodeKind string

const (
	NodeKindFile   NodeKind = "file"
	NodeKindFolder NodeKind = "folder"
)

type Entry struct {
	SourcePath string
	Name       string
	Kind       NodeKind
	Depth      int
}

type Proposal struct {
	SourcePath    string
	RelativePath  string
	OriginalName  string
	ProposedName  string
	Kind          NodeKind
	Depth         int
	Changed       bool
	RuleChainName string
	ParentDir     string
}
