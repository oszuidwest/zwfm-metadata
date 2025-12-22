package formatters

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"zwfm-metadata/core"
)

const maxRDSLength = 64

// Common patterns compiled once for reuse
var (
	parenRegex    = regexp.MustCompile(`\s*\([^)]*\)`)
	bracketRegex  = regexp.MustCompile(`\s*\[[^\]]*\]`)
	featRegex     = regexp.MustCompile(`(?i)\s+(feat\.?|ft\.?|featuring|with)\s+.+$`)
	ampFeatRegex  = regexp.MustCompile(`(?i)\s+&\s+.+$`)
	remixRegex    = regexp.MustCompile(`(?i)\s*[-–]\s*.*(Remix|Mix|Edit|Version|Instrumental|Acoustic|Live|Remaster|Radio).*$`)
	whitespaceReg = regexp.MustCompile(`\s+`)
)

// RDSFormatter formats metadata for RDS RadioText display with 64-character limit.
type RDSFormatter struct{}

// Format progressively simplifies text to fit the 64-character RDS limit.
func (r *RDSFormatter) Format(st *core.StructuredText) {
	st.Artist = cleanField(st.Artist)
	st.Title = cleanField(st.Title)

	if st.Len() <= maxRDSLength {
		return
	}

	st.Title = removeParentheses(st.Title)
	if st.Len() <= maxRDSLength {
		return
	}

	st.Artist = removeParentheses(st.Artist)
	if st.Len() <= maxRDSLength {
		return
	}

	st.Title = removeBrackets(st.Title)
	if st.Len() <= maxRDSLength {
		return
	}

	st.Artist = removeBrackets(st.Artist)
	if st.Len() <= maxRDSLength {
		return
	}

	st.Artist = removeFeaturing(st.Artist)
	if st.Len() <= maxRDSLength {
		return
	}

	st.Title = removeFeaturing(st.Title)
	if st.Len() <= maxRDSLength {
		return
	}

	st.Title = removeRemixIndicators(st.Title)
	if st.Len() <= maxRDSLength {
		return
	}

	smartTruncate(st)
}

// cleanField converts text to RDS-safe ASCII with normalized spacing.
func cleanField(s string) string {
	s = stripHTMLTags(s)
	s = transliterateToASCII(s)
	s = whitespaceReg.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func removeParentheses(s string) string {
	return strings.TrimSpace(parenRegex.ReplaceAllString(s, ""))
}

func removeBrackets(s string) string {
	return strings.TrimSpace(bracketRegex.ReplaceAllString(s, ""))
}

func removeFeaturing(s string) string {
	s = featRegex.ReplaceAllString(s, "")
	s = ampFeatRegex.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func removeRemixIndicators(s string) string {
	return strings.TrimSpace(remixRegex.ReplaceAllString(s, ""))
}

// smartTruncate shortens artist and title to fit, preserving artist when possible.
func smartTruncate(st *core.StructuredText) {
	overhead := utf8.RuneCountInString(st.Prefix) + utf8.RuneCountInString(st.Suffix)
	if st.Artist != "" && st.Title != "" {
		overhead += utf8.RuneCountInString(st.Separator)
	}

	available := maxRDSLength - overhead
	if available <= 0 {
		st.Artist = ""
		st.Title = ""
		return
	}

	artistLen := utf8.RuneCountInString(st.Artist)
	titleLen := utf8.RuneCountInString(st.Title)

	if artistLen+titleLen <= available {
		return
	}

	const minTitleLen = 10
	const ellipsis = "..."
	const ellipsisLen = 3

	if artistLen <= available-minTitleLen-ellipsisLen {
		maxTitle := available - artistLen - ellipsisLen
		st.Title = truncateAtWord(st.Title, maxTitle) + ellipsis
	} else if titleLen <= available-minTitleLen-ellipsisLen {
		maxArtist := available - titleLen - ellipsisLen
		st.Artist = truncateAtWord(st.Artist, maxArtist) + ellipsis
	} else {
		halfAvailable := (available - ellipsisLen*2) / 2
		st.Artist = truncateAtWord(st.Artist, halfAvailable) + ellipsis
		st.Title = truncateAtWord(st.Title, halfAvailable) + ellipsis
	}
}

// truncateAtWord shortens text to maxRunes, breaking at word boundaries when possible.
func truncateAtWord(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}

	truncated := runes[:maxRunes]
	for i := maxRunes - 1; i >= maxRunes-10 && i >= 0; i-- {
		if truncated[i] == ' ' || truncated[i] == '-' || truncated[i] == ',' {
			return strings.TrimSpace(string(truncated[:i]))
		}
	}

	return strings.TrimSpace(string(truncated))
}

func stripHTMLTags(text string) string {
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
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

	return strings.TrimSpace(result.String())
}

// transliterateToASCII converts characters to ASCII using Unicode normalization
// combined with explicit mappings for characters without NFD decompositions.
func transliterateToASCII(text string) string {
	text = expandMultiCharMappings(text)

	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		runes.Map(mapNonASCIIToASCII),
	)

	result, _, err := transform.String(t, text)
	if err != nil {
		return text
	}

	return result
}

// expandMultiCharMappings handles Unicode characters that expand to multiple ASCII characters.
func expandMultiCharMappings(text string) string {
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		switch r {
		case 'ß':
			result.WriteString("ss")
		case 'þ':
			result.WriteString("th")
		case 'Þ':
			result.WriteString("TH")
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
		case 'ﬅ', 'ﬆ':
			result.WriteString("st")
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

// mapNonASCIIToASCII maps single non-ASCII characters to their ASCII equivalents.
func mapNonASCIIToASCII(r rune) rune {
	if r <= 127 {
		return r
	}

	switch r {
	// Nordic/Scandinavian
	case 'ø':
		return 'o'
	case 'Ø':
		return 'O'
	case 'å':
		return 'a'
	case 'Å':
		return 'A'
	// Icelandic
	case 'ð':
		return 'd'
	case 'Ð':
		return 'D'
	// Slavic/Vietnamese
	case 'ł':
		return 'l'
	case 'Ł':
		return 'L'
	case 'đ':
		return 'd'
	case 'Đ':
		return 'D'
	case 'ħ':
		return 'h'
	case 'Ħ':
		return 'H'
	// Turkish
	case 'ı':
		return 'i'
	case 'İ':
		return 'I'
	case 'ş':
		return 's'
	case 'Ş':
		return 'S'
	case 'ğ':
		return 'g'
	case 'Ğ':
		return 'G'
	// Catalan
	case 'ŀ':
		return 'l'
	case 'Ŀ':
		return 'L'
	// Welsh
	case 'ŵ':
		return 'w'
	case 'Ŵ':
		return 'W'
	case 'ŷ':
		return 'y'
	case 'Ŷ':
		return 'Y'
	// Romanian/Latvian
	case 'ț':
		return 't'
	case 'Ț':
		return 'T'
	case 'ș':
		return 's'
	case 'Ș':
		return 'S'
	case 'ģ':
		return 'g'
	case 'Ģ':
		return 'G'
	case 'ķ':
		return 'k'
	case 'Ķ':
		return 'K'
	case 'ļ':
		return 'l'
	case 'Ļ':
		return 'L'
	case 'ņ':
		return 'n'
	case 'Ņ':
		return 'N'
	case 'ŗ':
		return 'r'
	case 'Ŗ':
		return 'R'
	// Czech/Slovak
	case 'ď':
		return 'd'
	case 'Ď':
		return 'D'
	case 'ť':
		return 't'
	case 'Ť':
		return 'T'
	case 'ň':
		return 'n'
	case 'Ň':
		return 'N'
	case 'ř':
		return 'r'
	case 'Ř':
		return 'R'
	case 'ů':
		return 'u'
	case 'Ů':
		return 'U'
	// Estonian
	case 'õ':
		return 'o'
	case 'Õ':
		return 'O'
	// Hungarian
	case 'ő':
		return 'o'
	case 'Ő':
		return 'O'
	case 'ű':
		return 'u'
	case 'Ű':
		return 'U'
	// Sami
	case 'ŋ':
		return 'n'
	case 'Ŋ':
		return 'N'
	case 'ŧ':
		return 't'
	case 'Ŧ':
		return 'T'
	// Esperanto
	case 'ĉ':
		return 'c'
	case 'Ĉ':
		return 'C'
	case 'ĝ':
		return 'g'
	case 'Ĝ':
		return 'G'
	case 'ĥ':
		return 'h'
	case 'Ĥ':
		return 'H'
	case 'ĵ':
		return 'j'
	case 'Ĵ':
		return 'J'
	case 'ŝ':
		return 's'
	case 'Ŝ':
		return 'S'
	case 'ŭ':
		return 'u'
	case 'Ŭ':
		return 'U'
	// Spanish/Basque
	case 'ñ':
		return 'n'
	case 'Ñ':
		return 'N'
	// Additional stroke variants
	case 'ƀ':
		return 'b'
	case 'Ƀ':
		return 'B'
	case 'ɉ':
		return 'j'
	case 'Ɉ':
		return 'J'
	case 'ƶ':
		return 'z'
	case 'Ƶ':
		return 'Z'
	// Latin Extended Additional
	case 'ḃ':
		return 'b'
	case 'Ḃ':
		return 'B'
	case 'ḋ':
		return 'd'
	case 'Ḋ':
		return 'D'
	case 'ḟ':
		return 'f'
	case 'Ḟ':
		return 'F'
	case 'ṁ':
		return 'm'
	case 'Ṁ':
		return 'M'
	case 'ṗ':
		return 'p'
	case 'Ṗ':
		return 'P'
	case 'ṡ':
		return 's'
	case 'Ṡ':
		return 'S'
	case 'ṫ':
		return 't'
	case 'Ṫ':
		return 'T'
	default:
		return -1
	}
}

func init() {
	RegisterFormatter("rds", func() core.Formatter { return &RDSFormatter{} })
}
