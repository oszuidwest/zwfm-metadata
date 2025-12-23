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

// multiCharMappings maps Unicode characters that expand to multiple ASCII characters.
var multiCharMappings = map[rune]string{
	'ß': "ss", 'þ': "th", 'Þ': "TH",
	'æ': "ae", 'Æ': "AE", 'œ': "oe", 'Œ': "OE",
	'ĳ': "ij", 'Ĳ': "IJ",
	'ﬁ': "fi", 'ﬂ': "fl", 'ﬀ': "ff", 'ﬃ': "ffi", 'ﬄ': "ffl",
	'ﬅ': "st", 'ﬆ': "st",
	'ǈ': "lj", 'ǉ': "Lj", 'Ǉ': "LJ",
	'ǋ': "nj", 'ǌ': "Nj", 'Ǌ': "NJ",
	'ǅ': "dz", 'ǆ': "Dz", 'Ǆ': "DZ",
}

// nonASCIIToASCII maps single non-ASCII characters to their ASCII equivalents.
var nonASCIIToASCII = map[rune]rune{
	// Nordic/Scandinavian
	'ø': 'o', 'Ø': 'O', 'å': 'a', 'Å': 'A',
	// Icelandic
	'ð': 'd', 'Ð': 'D',
	// Slavic/Vietnamese
	'ł': 'l', 'Ł': 'L', 'đ': 'd', 'Đ': 'D', 'ħ': 'h', 'Ħ': 'H',
	// Turkish
	'ı': 'i', 'İ': 'I', 'ş': 's', 'Ş': 'S', 'ğ': 'g', 'Ğ': 'G',
	// Catalan
	'ŀ': 'l', 'Ŀ': 'L',
	// Welsh
	'ŵ': 'w', 'Ŵ': 'W', 'ŷ': 'y', 'Ŷ': 'Y',
	// Romanian/Latvian
	'ț': 't', 'Ț': 'T', 'ș': 's', 'Ș': 'S',
	'ģ': 'g', 'Ģ': 'G', 'ķ': 'k', 'Ķ': 'K',
	'ļ': 'l', 'Ļ': 'L', 'ņ': 'n', 'Ņ': 'N', 'ŗ': 'r', 'Ŗ': 'R',
	// Czech/Slovak
	'ď': 'd', 'Ď': 'D', 'ť': 't', 'Ť': 'T',
	'ň': 'n', 'Ň': 'N', 'ř': 'r', 'Ř': 'R', 'ů': 'u', 'Ů': 'U',
	// Estonian
	'õ': 'o', 'Õ': 'O',
	// Hungarian
	'ő': 'o', 'Ő': 'O', 'ű': 'u', 'Ű': 'U',
	// Sami
	'ŋ': 'n', 'Ŋ': 'N', 'ŧ': 't', 'Ŧ': 'T',
	// Esperanto
	'ĉ': 'c', 'Ĉ': 'C', 'ĝ': 'g', 'Ĝ': 'G',
	'ĥ': 'h', 'Ĥ': 'H', 'ĵ': 'j', 'Ĵ': 'J',
	'ŝ': 's', 'Ŝ': 'S', 'ŭ': 'u', 'Ŭ': 'U',
	// Spanish/Basque
	'ñ': 'n', 'Ñ': 'N',
	// Additional stroke variants
	'ƀ': 'b', 'Ƀ': 'B', 'ɉ': 'j', 'Ɉ': 'J', 'ƶ': 'z', 'Ƶ': 'Z',
	// Latin Extended Additional
	'ḃ': 'b', 'Ḃ': 'B', 'ḋ': 'd', 'Ḋ': 'D',
	'ḟ': 'f', 'Ḟ': 'F', 'ṁ': 'm', 'Ṁ': 'M',
	'ṗ': 'p', 'Ṗ': 'P', 'ṡ': 's', 'Ṡ': 'S', 'ṫ': 't', 'Ṫ': 'T',
}

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

	switch {
	case artistLen <= available-minTitleLen-ellipsisLen:
		maxTitle := available - artistLen - ellipsisLen
		st.Title = truncateAtWord(st.Title, maxTitle) + ellipsis
	case titleLen <= available-minTitleLen-ellipsisLen:
		maxArtist := available - titleLen - ellipsisLen
		st.Artist = truncateAtWord(st.Artist, maxArtist) + ellipsis
	default:
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

	chars := []rune(s)
	if len(chars) <= maxRunes {
		return s
	}

	truncated := chars[:maxRunes]
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
		if mapped, ok := multiCharMappings[r]; ok {
			result.WriteString(mapped)
		} else {
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
	if mapped, ok := nonASCIIToASCII[r]; ok {
		return mapped
	}
	return -1
}

func init() {
	RegisterFormatter("rds", func() core.Formatter { return &RDSFormatter{} })
}
