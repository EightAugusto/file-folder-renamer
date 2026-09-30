package rename

import "testing"

func TestParseSeparatesStemAndExtension(t *testing.T) {
	testCases := []struct {
		name              string
		input             string
		expectedStem      string
		expectedExtension string
		expectedProtected bool
	}{
		{name: "simple extension", input: "report.pdf", expectedStem: "report", expectedExtension: ".pdf"},
		{name: "compound tar gzip", input: "archive.tar.gz", expectedStem: "archive", expectedExtension: ".tar.gz"},
		{name: "compound tar gzip preserves case", input: "ARCHIVE.TAR.GZ", expectedStem: "ARCHIVE", expectedExtension: ".TAR.GZ"},
		{name: "compound tar bzip", input: "archive.tar.bz2", expectedStem: "archive", expectedExtension: ".tar.bz2"},
		{name: "compound tar xz", input: "archive.tar.xz", expectedStem: "archive", expectedExtension: ".tar.xz"},
		{name: "compound tar zstd", input: "archive.tar.zst", expectedStem: "archive", expectedExtension: ".tar.zst"},
		{name: "unrecognized dotted name", input: "john.smith.resume.pdf", expectedStem: "john.smith.resume", expectedExtension: ".pdf"},
		{name: "protected dotfile", input: ".gitignore", expectedExtension: ".gitignore", expectedProtected: true},
		{name: "protected dotfile with suffix", input: ".env.local", expectedExtension: ".env.local", expectedProtected: true},
		{name: "duplicate boundary dots", input: "..hidden..file", expectedStem: "hidden", expectedExtension: ".file"},
		{name: "no extension", input: "README", expectedStem: "README"},
		{name: "trailing dot", input: "file.", expectedStem: "file."},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			parts := Parse(testCase.input)
			if parts.Stem != testCase.expectedStem {
				t.Fatalf("stem: got %q, want %q", parts.Stem, testCase.expectedStem)
			}
			if parts.Extension != testCase.expectedExtension {
				t.Fatalf("extension: got %q, want %q", parts.Extension, testCase.expectedExtension)
			}
			if parts.Protected != testCase.expectedProtected {
				t.Fatalf("protected: got %t, want %t", parts.Protected, testCase.expectedProtected)
			}
		})
	}
}

func TestNormalizeExtensionLowercasesTheFullSuffix(t *testing.T) {
	if actual := NormalizeExtension(".TAR.GZ"); actual != ".tar.gz" {
		t.Fatalf("got %q", actual)
	}
}
