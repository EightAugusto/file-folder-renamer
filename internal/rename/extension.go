package rename

import "strings"

var compoundExtensions = []string{
	".tar.bz2",
	".tar.gz",
	".tar.xz",
	".tar.zst",
}

// Parse separates a path into a basename, renameable stem, and preserved extension.
func Parse(path string) Parts {
	baseName := Base(path)
	if isProtectedDotfile(baseName) {
		return Parts{BaseName: baseName, Extension: baseName, Protected: true}
	}

	if extension := compoundExtension(baseName); extension != "" {
		stem := trimBoundaryDots(baseName[:len(baseName)-len(extension)])
		return Parts{
			BaseName:  baseName,
			Stem:      stem,
			Extension: baseName[len(baseName)-len(extension):],
		}
	}

	extensionIndex := strings.LastIndex(baseName, ".")
	if extensionIndex <= 0 || extensionIndex == len(baseName)-1 {
		return Parts{BaseName: baseName, Stem: baseName}
	}
	return Parts{
		BaseName:  baseName,
		Stem:      trimBoundaryDots(baseName[:extensionIndex]),
		Extension: baseName[extensionIndex:],
	}
}

func trimBoundaryDots(stem string) string {
	return strings.Trim(stem, ".")
}

// NormalizeExtension applies the application-wide extension casing policy.
func NormalizeExtension(extension string) string {
	return strings.ToLower(extension)
}

func compoundExtension(baseName string) string {
	normalizedName := strings.ToLower(baseName)
	for _, extension := range compoundExtensions {
		if len(baseName) > len(extension) && strings.HasSuffix(normalizedName, extension) {
			return extension
		}
	}
	return ""
}

func isProtectedDotfile(baseName string) bool {
	return strings.HasPrefix(baseName, ".") && !strings.HasPrefix(baseName, "..")
}
