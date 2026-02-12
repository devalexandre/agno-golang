package tools

import (
	"strings"

	"golang.org/x/net/html"
)

// htmlExtractTitle extracts the <title> tag text from an HTML document.
func htmlExtractTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		if n.FirstChild != nil {
			return n.FirstChild.Data
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if title := htmlExtractTitle(c); title != "" {
			return title
		}
	}
	return ""
}

// htmlExtractTextContent extracts and cleans text content from an HTML node,
// skipping script, style, nav, footer, and header elements.
func htmlExtractTextContent(n *html.Node) string {
	var sb strings.Builder
	htmlExtractText(n, &sb)
	text := sb.String()
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

func htmlExtractText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "noscript", "nav", "footer", "header":
			return
		}
	}
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			sb.WriteString(text)
			sb.WriteString("\n")
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		htmlExtractText(c, sb)
	}
}

// htmlExtractLinks extracts all absolute links from an HTML document.
func htmlExtractLinks(n *html.Node, baseURL string) []string {
	var links []string
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				href := attr.Val
				if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
					links = append(links, href)
				} else if strings.HasPrefix(href, "/") {
					base := baseURL
					if idx := strings.Index(base[8:], "/"); idx >= 0 {
						base = base[:idx+8]
					}
					links = append(links, base+href)
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		links = append(links, htmlExtractLinks(c, baseURL)...)
	}
	return links
}

// htmlExtractMetaContent extracts the content attribute from a <meta> tag
// matching the given property or name.
func htmlExtractMetaContent(n *html.Node, property string) string {
	if n.Type == html.ElementNode && n.Data == "meta" {
		var prop, content string
		for _, attr := range n.Attr {
			if (attr.Key == "property" || attr.Key == "name") && attr.Val == property {
				prop = attr.Val
			}
			if attr.Key == "content" {
				content = attr.Val
			}
		}
		if prop != "" && content != "" {
			return content
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := htmlExtractMetaContent(c, property); result != "" {
			return result
		}
	}
	return ""
}

// htmlFindElement finds the first element with the given tag name.
func htmlFindElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := htmlFindElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

// htmlExtractParagraphs extracts all <p> text content as joined paragraphs.
func htmlExtractParagraphs(n *html.Node) string {
	var paragraphs []string
	htmlCollectParagraphs(n, &paragraphs)
	return strings.Join(paragraphs, "\n\n")
}

func htmlCollectParagraphs(n *html.Node, paragraphs *[]string) {
	if n.Type == html.ElementNode && n.Data == "p" {
		var sb strings.Builder
		htmlCollectInlineText(n, &sb)
		text := strings.TrimSpace(sb.String())
		if text != "" {
			*paragraphs = append(*paragraphs, text)
		}
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		htmlCollectParagraphs(c, paragraphs)
	}
}

func htmlCollectInlineText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		htmlCollectInlineText(c, sb)
	}
}

// htmlExtractAllImages extracts all <img> src URLs that start with http/https.
func htmlExtractAllImages(n *html.Node) []string {
	var images []string
	if n.Type == html.ElementNode && n.Data == "img" {
		for _, attr := range n.Attr {
			if attr.Key == "src" && (strings.HasPrefix(attr.Val, "http://") || strings.HasPrefix(attr.Val, "https://")) {
				images = append(images, attr.Val)
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		images = append(images, htmlExtractAllImages(c)...)
	}
	return images
}
