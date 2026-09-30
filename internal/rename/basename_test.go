package rename

import "testing"

func TestBaseSupportsUnixAndWindowsPaths(t *testing.T) {
	testCases := []struct {
		path     string
		expected string
	}{
		{path: "/home/john.v2/archive.tar.gz", expected: "archive.tar.gz"},
		{path: `C:\Users\john.v2\archive.tar.gz`, expected: "archive.tar.gz"},
		{path: "archive.tar.gz", expected: "archive.tar.gz"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.path, func(t *testing.T) {
			if actual := Base(testCase.path); actual != testCase.expected {
				t.Fatalf("got %q, want %q", actual, testCase.expected)
			}
		})
	}
}
