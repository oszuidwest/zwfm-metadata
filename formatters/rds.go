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
	processedText := text

	// 1. Remove content in parentheses progressively from right to left
	// This preserves more important information that might be in earlier parentheses
	for {
		// Find the rightmost parentheses
		re := regexp.MustCompile(`\s*\([^)]+\)`)
		matches := re.FindAllStringIndex(processedText, -1)
		if len(matches) == 0 {
			break
		}
		
		// Remove the rightmost match
		lastMatch := matches[len(matches)-1]
		testText := processedText[:lastMatch[0]] + processedText[lastMatch[1]:]
		testText = strings.TrimSpace(testText)
		
		// If removing this makes it fit, we're done
		if len(testText) <= 64 {
			return testText
		}
		
		// Otherwise, remove it and continue
		processedText = testText
	}

	// 2. Remove content in brackets progressively from right to left
	for {
		// Find the rightmost brackets
		re := regexp.MustCompile(`\s*\[[^\]]+\]`)
		matches := re.FindAllStringIndex(processedText, -1)
		if len(matches) == 0 {
			break
		}
		
		// Remove the rightmost match
		lastMatch := matches[len(matches)-1]
		testText := processedText[:lastMatch[0]] + processedText[lastMatch[1]:]
		testText = strings.TrimSpace(testText)
		
		// If removing this makes it fit, we're done
		if len(testText) <= 64 {
			return testText
		}
		
		// Otherwise, remove it and continue
		processedText = testText
	}

	// 3. Remove featured artists (various patterns)
	// Check if we have artist - title format
	if strings.Contains(processedText, " - ") {
		parts := strings.SplitN(processedText, " - ", 2)
		if len(parts) == 2 {
			// Remove featured artists from artist name
			parts[0] = regexp.MustCompile(`(?i)\s+(feat\.|ft\.|featuring|with)\s+.+$`).ReplaceAllString(parts[0], "")
			parts[0] = regexp.MustCompile(`(?i)\s+&\s+.+$`).ReplaceAllString(parts[0], "")

			// Remove featured artists from title
			parts[1] = regexp.MustCompile(`(?i)\s+(feat\.|ft\.|featuring|with)\s+.+$`).ReplaceAllString(parts[1], "")

			processedText = strings.TrimSpace(parts[0]) + " - " + strings.TrimSpace(parts[1])
			if len(processedText) <= 64 {
				return processedText
			}
		}
	} else {
		// No separator, remove featured artists from whole string
		processedText = regexp.MustCompile(`(?i)\s+(feat\.|ft\.|featuring|with)\s+.+$`).ReplaceAllString(processedText, "")
		if len(processedText) <= 64 {
			return strings.TrimSpace(processedText)
		}
	}

	// 4. Remove remix/version indicators
	// First try to remove everything after a second hyphen (common for remixes)
	if strings.Count(processedText, " - ") >= 2 {
		// Find the second " - " and remove everything after it
		firstDash := strings.Index(processedText, " - ")
		if firstDash >= 0 {
			secondDash := strings.Index(processedText[firstDash+3:], " - ")
			if secondDash >= 0 {
				testProcessed := processedText[:firstDash+3+secondDash]
				if len(testProcessed) <= 64 {
					return strings.TrimSpace(testProcessed)
				}
			}
		}
	}

	// Try removing common suffixes
	remixPattern := regexp.MustCompile(`(?i)\s*[-–]\s*.*(Remix|Mix|Edit|Version|Instrumental|Acoustic|Live|Remaster|Radio).*$`)
	testProcessed := remixPattern.ReplaceAllString(processedText, "")
	if len(testProcessed) <= 64 && len(testProcessed) > 0 {
		return strings.TrimSpace(testProcessed)
	}

	// 5. If still too long, truncate intelligently
	// Try to cut at a natural boundary (space, hyphen, comma)
	if len(processedText) > 64 {
		truncated := processedText[:61] + "..."

		// Look for a better cut point
		for i := 60; i >= 50; i-- {
			if processedText[i] == ' ' || processedText[i] == '-' || processedText[i] == ',' {
				truncated = strings.TrimSpace(processedText[:i]) + "..."
				break
			}
		}

		return truncated
	}

	return processedText
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

func init() {
	RegisterFormatter("rds", func() Formatter { return &RDSFormatter{} })
}
