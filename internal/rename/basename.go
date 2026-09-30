package rename

import "strings"

// Base extracts the final path element from Unix or Windows path strings.
func Base(path string) string {
	separatorIndex := strings.LastIndexAny(path, "/\\")
	if separatorIndex == -1 {
		return path
	}
	return path[separatorIndex+1:]
}
