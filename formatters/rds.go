package formatters

import (
	"golang.org/x/net/html"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"regexp"
	"strings"
	"unicode"
)

// RDSFormatter truncates text to 64 characters with smart simplification for radio display.
type RDSFormatter struct{}

// Format applies progressive simplification to fit text within 64 characters.
func (r *RDSFormatter) Format(text string) string {
	text = stripHTMLTags(text)

	if len(text) <= 64 {
		return text
	}

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

func stripHTMLTags(text string) string {
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
		// If parsing fails, return original text
		return text
	}

	result := extractText(doc)
	return filterVisibleText(result)
}

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

func filterVisibleText(text string) string {
	text = transliterateToASCII(text)

	var result strings.Builder
	for _, r := range text {
		if (r >= 32 && r <= 126) || r == ' ' {
			result.WriteRune(r)
		} else if r == '\n' || r == '\r' || r == '\t' {
			result.WriteRune(' ')
		}
	}

	text = regexp.MustCompile(`\s+`).ReplaceAllString(result.String(), " ")
	return strings.TrimSpace(text)
}

// transliterateToASCII converts characters to ASCII using Unicode normalization
// combined with explicit mappings for characters that don't have NFD decompositions.
//
// NOTE: This transliteration is a temporary workaround for a bug in StereoTool's RDS
// implementation that doesn't properly handle extended Latin characters (0x80-0xFF range).
// While these characters are part of the EBU Latin character set and should be valid for
// RDS, StereoTool fails to display them correctly. This function ensures all characters
// are pure ASCII (0x00-0x7F) to work around this limitation.
//
// Once StereoTool properly supports the full EBU Latin character set, this aggressive
// ASCII-only transliteration should be removed.
func transliterateToASCII(text string) string {
	// First pass: handle multi-character expansions (one Unicode char → multiple ASCII)
	text = expandMultiCharMappings(text)

	// Second pass: Use transform.Chain for the rest:
	// 1. NFD - Canonical Decomposition (é → e + combining acute accent)
	// 2. Remove combining marks (unicode.Mn category - all diacritics/accents)
	// 3. Map remaining non-ASCII base characters using explicit table
	t := transform.Chain(
		norm.NFD,                           // Unicode NFD normalization
		runes.Remove(runes.In(unicode.Mn)), // Remove combining marks (diacritics)
		runes.Map(mapNonASCIIToASCII),      // Handle base chars without NFD decompositions
	)

	result, _, err := transform.String(t, text)
	if err != nil {
		// If transformation fails, return original text
		return text
	}

	return result
}

// expandMultiCharMappings handles Unicode characters that expand to multiple ASCII characters.
// These must be handled separately since runes.Map only supports 1:1 mappings.
func expandMultiCharMappings(text string) string {
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		switch r {
		// Germanic
		case 'ß':
			result.WriteString("ss")

		// Icelandic thorn (old English "th")
		case 'þ':
			result.WriteString("th")
		case 'Þ':
			result.WriteString("TH")

		// Ligatures
		case 'æ':
			result.WriteString("ae")
		case 'Æ':
			result.WriteString("AE")
		case 'œ':
			result.WriteString("oe")
		case 'Œ':
			result.WriteString("OE")
		case 'ĳ':
			result.WriteString("ij")
		case 'Ĳ':
			result.WriteString("IJ")

		// Typographic ligatures
		case 'ﬁ':
			result.WriteString("fi")
		case 'ﬂ':
			result.WriteString("fl")
		case 'ﬀ':
			result.WriteString("ff")
		case 'ﬃ':
			result.WriteString("ffi")
		case 'ﬄ':
			result.WriteString("ffl")
		case 'ﬅ':
			result.WriteString("st")
		case 'ﬆ':
			result.WriteString("st")

		// Croatian digraphs
		case 'ǈ':
			result.WriteString("lj")
		case 'ǉ':
			result.WriteString("Lj")
		case 'Ǉ':
			result.WriteString("LJ")
		case 'ǋ':
			result.WriteString("nj")
		case 'ǌ':
			result.WriteString("Nj")
		case 'Ǌ':
			result.WriteString("NJ")
		case 'ǅ':
			result.WriteString("dz")
		case 'ǆ':
			result.WriteString("Dz")
		case 'Ǆ':
			result.WriteString("DZ")

		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// mapNonASCIIToASCII maps single non-ASCII base characters to their ASCII equivalents.
// These are characters that have no Unicode NFD decomposition and must be handled explicitly.
// Returns -1 to remove unknown non-ASCII characters.
func mapNonASCIIToASCII(r rune) rune {
	// Already ASCII - pass through
	if r <= 127 {
		return r
	}

	// Explicit mappings for base characters without NFD decompositions
	switch r {
	// Nordic/Scandinavian
	case 'ø':
		return 'o' // Danish/Norwegian o with stroke
	case 'Ø':
		return 'O'
	case 'å':
		return 'a' // Scandinavian a with ring above
	case 'Å':
		return 'A'

	// Icelandic
	case 'ð':
		return 'd' // Eth (lowercase)
	case 'Ð':
		return 'D' // Eth (uppercase)

	// Slavic/Vietnamese (đ/Đ used in both Croatian/Serbian and Vietnamese)
	case 'ł':
		return 'l' // Polish l with stroke
	case 'Ł':
		return 'L'
	case 'đ':
		return 'd' // Croatian/Serbian/Vietnamese d with stroke
	case 'Đ':
		return 'D'
	case 'ħ':
		return 'h' // Maltese h with stroke
	case 'Ħ':
		return 'H'

	// Turkish
	case 'ı':
		return 'i' // Dotless i
	case 'İ':
		return 'I' // I with dot above
	case 'ş':
		return 's' // S with cedilla
	case 'Ş':
		return 'S'
	case 'ğ':
		return 'g' // G with breve
	case 'Ğ':
		return 'G'

	// Catalan
	case 'ŀ':
		return 'l' // L with middle dot
	case 'Ŀ':
		return 'L'

	// Welsh
	case 'ŵ':
		return 'w' // W with circumflex
	case 'Ŵ':
		return 'W'
	case 'ŷ':
		return 'y' // Y with circumflex
	case 'Ŷ':
		return 'Y'

	// Romanian/Latvian
	case 'ț':
		return 't' // T with comma below
	case 'Ț':
		return 'T'
	case 'ș':
		return 's' // S with comma below
	case 'Ș':
		return 'S'
	case 'ģ':
		return 'g' // G with cedilla
	case 'Ģ':
		return 'G'
	case 'ķ':
		return 'k' // K with cedilla
	case 'Ķ':
		return 'K'
	case 'ļ':
		return 'l' // L with cedilla
	case 'Ļ':
		return 'L'
	case 'ņ':
		return 'n' // N with cedilla
	case 'Ņ':
		return 'N'
	case 'ŗ':
		return 'r' // R with cedilla
	case 'Ŗ':
		return 'R'

	// Czech/Slovak
	case 'ď':
		return 'd' // D with caron
	case 'Ď':
		return 'D'
	case 'ť':
		return 't' // T with caron
	case 'Ť':
		return 'T'
	case 'ň':
		return 'n' // N with caron
	case 'Ň':
		return 'N'
	case 'ř':
		return 'r' // R with caron
	case 'Ř':
		return 'R'
	case 'ů':
		return 'u' // U with ring above
	case 'Ů':
		return 'U'

	// Estonian
	case 'õ':
		return 'o' // O with tilde
	case 'Õ':
		return 'O'

	// Hungarian
	case 'ő':
		return 'o' // O with double acute
	case 'Ő':
		return 'O'
	case 'ű':
		return 'u' // U with double acute
	case 'Ű':
		return 'U'

	// Sami languages
	case 'ŋ':
		return 'n' // Eng (velar nasal)
	case 'Ŋ':
		return 'N'
	case 'ŧ':
		return 't' // T with stroke
	case 'Ŧ':
		return 'T'

	// Esperanto
	case 'ĉ':
		return 'c' // C with circumflex
	case 'Ĉ':
		return 'C'
	case 'ĝ':
		return 'g' // G with circumflex
	case 'Ĝ':
		return 'G'
	case 'ĥ':
		return 'h' // H with circumflex
	case 'Ĥ':
		return 'H'
	case 'ĵ':
		return 'j' // J with circumflex
	case 'Ĵ':
		return 'J'
	case 'ŝ':
		return 's' // S with circumflex
	case 'Ŝ':
		return 'S'
	case 'ŭ':
		return 'u' // U with breve
	case 'Ŭ':
		return 'U'

	// Basque
	case 'ñ':
		return 'n' // N with tilde (also Spanish)
	case 'Ñ':
		return 'N'

	// Additional stroke variants
	case 'ƀ':
		return 'b' // B with stroke
	case 'Ƀ':
		return 'B'
	case 'ɉ':
		return 'j' // J with stroke
	case 'Ɉ':
		return 'J'
	case 'ƶ':
		return 'z' // Z with stroke
	case 'Ƶ':
		return 'Z'

	// Latin Extended Additional
	case 'ḃ':
		return 'b' // B with dot above
	case 'Ḃ':
		return 'B'
	case 'ḋ':
		return 'd' // D with dot above
	case 'Ḋ':
		return 'D'
	case 'ḟ':
		return 'f' // F with dot above
	case 'Ḟ':
		return 'F'
	case 'ṁ':
		return 'm' // M with dot above
	case 'Ṁ':
		return 'M'
	case 'ṗ':
		return 'p' // P with dot above
	case 'Ṗ':
		return 'P'
	case 'ṡ':
		return 's' // S with dot above
	case 'Ṡ':
		return 'S'
	case 'ṫ':
		return 't' // T with dot above
	case 'Ṫ':
		return 'T'

	default:
		// Remove any remaining non-ASCII characters
		return -1
	}
}

func init() {
	RegisterFormatter("rds", func() Formatter { return &RDSFormatter{} })
}
