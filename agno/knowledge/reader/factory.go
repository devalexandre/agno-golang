package reader

import (
	"strings"

	"golang.org/x/net/html"
)

// ReaderForExtension returns a file-based Reader appropriate for the given file
// extension (case-insensitive, leading dot optional), or nil if none matches.
// Network/query readers (website, wikipedia, arxiv, youtube) are not returned
// here since they are not selected by file extension.
func ReaderForExtension(ext string) Reader {
	ext = strings.ToLower(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	switch ext {
	case ".txt", ".text", ".log":
		return NewTextReader()
	case ".md", ".markdown", ".mdx":
		return NewMarkdownReader()
	case ".csv", ".tsv":
		return NewCSVReader()
	case ".docx":
		return NewDocxReader()
	default:
		return nil
	}
}

// parseHTMLString parses an HTML document from a string (test/helper utility).
func parseHTMLString(s string) (*html.Node, error) {
	return html.Parse(strings.NewReader(s))
}
