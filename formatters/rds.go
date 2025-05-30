package formatters

import (
	"golang.org/x/net/html"
	"regexp"
	"strings"
	"unicode"
)

// RDSFormatter limits text to 64 characters for RDS compliance
type RDSFormatter struct{}

// Format implements the Formatter interface with smart truncation
func (r *RDSFormatter) Format(text string) string {
	// First strip HTML tags and decode entities for plain text
	text = stripHTMLTags(text)

	// If already within limit, return as is
	if len(text) <= 64 {
		return text
	}

	// Try progressive simplification strategies
	simplified := text

	// 1. Remove content in parentheses (e.g., "(Radio Edit)", "(2024 Remaster)")
	simplified = regexp.MustCompile(`\s*\([^)]+\)`).ReplaceAllString(simplified, "")
	if len(simplified) <= 64 {
		return strings.TrimSpace(simplified)
	}

	// 2. Remove content in brackets (e.g., "[Live]", "[Explicit]")
	simplified = regexp.MustCompile(`\s*\[[^\]]+\]`).ReplaceAllString(simplified, "")
	if len(simplified) <= 64 {
		return strings.TrimSpace(simplified)
	}

	// 3. Remove featured artists (various patterns)
	// Check if we have artist - title format
	if strings.Contains(simplified, " - ") {
		parts := strings.SplitN(simplified, " - ", 2)
		if len(parts) == 2 {
			// Remove featured artists from artist name
			parts[0] = regexp.MustCompile(`(?i)\s+(feat\.|ft\.|featuring|with)\s+.+$`).ReplaceAllString(parts[0], "")
			parts[0] = regexp.MustCompile(`(?i)\s+&\s+.+$`).ReplaceAllString(parts[0], "")

			// Remove featured artists from title
			parts[1] = regexp.MustCompile(`(?i)\s+(feat\.|ft\.|featuring|with)\s+.+$`).ReplaceAllString(parts[1], "")

			simplified = strings.TrimSpace(parts[0]) + " - " + strings.TrimSpace(parts[1])
			if len(simplified) <= 64 {
				return simplified
			}
		}
	} else {
		// No separator, remove featured artists from whole string
		simplified = regexp.MustCompile(`(?i)\s+(feat\.|ft\.|featuring|with)\s+.+$`).ReplaceAllString(simplified, "")
		if len(simplified) <= 64 {
			return strings.TrimSpace(simplified)
		}
	}

	// 4. Remove remix/version indicators
	// First try to remove everything after a second hyphen (common for remixes)
	if strings.Count(simplified, " - ") >= 2 {
		// Find the second " - " and remove everything after it
		firstDash := strings.Index(simplified, " - ")
		if firstDash >= 0 {
			secondDash := strings.Index(simplified[firstDash+3:], " - ")
			if secondDash >= 0 {
				testSimplified := simplified[:firstDash+3+secondDash]
				if len(testSimplified) <= 64 {
					return strings.TrimSpace(testSimplified)
				}
			}
		}
	}

	// Try removing common suffixes
	remixPattern := regexp.MustCompile(`(?i)\s*[-–]\s*.*(Remix|Mix|Edit|Version|Instrumental|Acoustic|Live|Remaster|Radio).*$`)
	testSimplified := remixPattern.ReplaceAllString(simplified, "")
	if len(testSimplified) <= 64 && len(testSimplified) > 0 {
		return strings.TrimSpace(testSimplified)
	}

	// 5. If still too long, truncate intelligently
	// Try to cut at a natural boundary (space, hyphen, comma)
	if len(simplified) > 64 {
		truncated := simplified[:61] + "..."

		// Look for a better cut point
		for i := 60; i >= 50; i-- {
			if simplified[i] == ' ' || simplified[i] == '-' || simplified[i] == ',' {
				truncated = strings.TrimSpace(simplified[:i]) + "..."
				break
			}
		}

		return truncated
	}

	return simplified
}

// stripHTMLTags removes all HTML tags and decodes entities
func stripHTMLTags(text string) string {
	// Parse HTML and extract text content
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
		// If parsing fails, return original text
		return text
	}

	result := extractText(doc)

	// Filter out invisible/control characters and convert newlines for RDS displays
	return filterVisibleText(result)
}

// extractText recursively extracts text content from HTML nodes
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	var result strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result.WriteString(extractText(c))
	}

	return result.String()
}

// filterVisibleText removes invisible and control characters, keeping only printable text
func filterVisibleText(text string) string {
	var result strings.Builder
	for _, r := range text {
		// Keep printable characters and basic whitespace (space, tab) but exclude newlines
		if unicode.IsPrint(r) || r == ' ' || r == '\t' {
			result.WriteRune(r)
		} else if r == '\n' || r == '\r' {
			// Convert newlines to spaces for single-line RDS output
			result.WriteRune(' ')
		}
	}
	return result.String()
}
