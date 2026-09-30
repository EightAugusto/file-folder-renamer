package rename

// Parts contains the filename components used by rename proposals.
type Parts struct {
	BaseName  string
	Stem      string
	Extension string
	Protected bool
}
