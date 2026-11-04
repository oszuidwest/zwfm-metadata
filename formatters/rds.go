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

// rdsCharset holds the non-ASCII characters of the basic RDS character set (IEC 62106-4
// Table 4) that Stereo Tool 11.06 puts on air with the code from that table, measured
// on its composite output. One row per table column. Other characters are transliterated.
const rdsCharset = "áàéèíìóòúùÑÇŞßĲ" +
	"âäêëîïôöûüñçşğĳ" +
	"α©‰Ğěňőπ£" +
	"¹²³±İńűµ¿÷°¼½¾§" +
	"ÁÀÉÈÍÌÓÒÚÙŘČŠŽĐĿ" +
	"ÂÄÊËÎÏÔÖÛÜřčšžđŀ" +
	"ÃÅÆŒÝÕØÞŊŔĆŚŹŦ" +
	"ãåæœŵýõøþŋŕćśźŧ"

var (
	parenRegex   = regexp.MustCompile(`\s*\([^)]*\)`)
	bracketRegex = regexp.MustCompile(`\s*\[[^\]]*\]`)
	featRegex    = regexp.MustCompile(`(?i)\s+(feat\.?|ft\.?|featuring|with)\s+.+$`)
	ampFeatRegex = regexp.MustCompile(`(?i)\s+&\s+.+$`)
	remixRegex   = regexp.MustCompile(`(?i)\s*[-–]\s*.*(Remix|Mix|Edit|Version|Instrumental|Acoustic|Live|Remaster|Radio).*$`)
)

// multiCharMappings maps Unicode characters that expand to multiple ASCII characters.
var multiCharMappings = map[rune]string{
	'ﬁ': "fi", 'ﬂ': "fl", 'ﬀ': "ff", 'ﬃ': "ffi", 'ﬄ': "ffl",
	'…': "...", '€': "EUR",
	'ﬅ': "st", 'ﬆ': "st",
	'ǈ': "Lj", 'ǉ': "lj", 'Ǉ': "LJ",
	'ǋ': "Nj", 'ǌ': "nj", 'Ǌ': "NJ",
	'ǅ': "Dz", 'ǆ': "dz", 'Ǆ': "DZ",
}

var multiCharReplacer = func() *strings.Replacer {
	pairs := make([]string, 0, 2*len(multiCharMappings))
	for r, mapped := range multiCharMappings {
		pairs = append(pairs, string(r), mapped)
	}
	return strings.NewReplacer(pairs...)
}()

var nonASCIIToASCII = map[rune]rune{
	// Punctuation
	'‘': '\'', '’': '\'', '‚': '\'', '‛': '\'',
	'“': '"', '”': '"', '„': '"',
	'‐': '-', '‑': '-', '–': '-', '—': '-', '―': '-',
	// Spaces
	'\u00a0': ' ', '\u2002': ' ', '\u2003': ' ', '\u2009': ' ', '\u200a': ' ', '\u202f': ' ',
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

// Format transforms structured text to fit within RDS RadioText constraints.
// Shortening steps run in order until the text fits; truncation is the last resort.
func (r *RDSFormatter) Format(st *core.StructuredText) {
	st.Artist = cleanField(st.Artist)
	st.Title = cleanField(st.Title)
	st.Prefix = cleanAffix(st.Prefix)
	st.Suffix = cleanAffix(st.Suffix)

	steps := []struct {
		field *string
		re    *regexp.Regexp
	}{
		{&st.Title, parenRegex},
		{&st.Artist, parenRegex},
		{&st.Title, bracketRegex},
		{&st.Artist, bracketRegex},
		{&st.Artist, featRegex},
		{&st.Artist, ampFeatRegex},
		{&st.Title, featRegex},
		{&st.Title, ampFeatRegex},
		{&st.Title, remixRegex},
	}

	for _, step := range steps {
		if st.Len() <= maxRDSLength {
			return
		}
		*step.field = strings.TrimSpace(step.re.ReplaceAllString(*step.field, ""))
	}

	if st.Len() > maxRDSLength {
		smartTruncate(st)
	}
}

// cleanField converts text to the RDS character set with normalized spacing.
func cleanField(s string) string {
	return strings.Join(strings.Fields(stripHTMLTags(s)), " ")
}

// cleanAffix cleans a prefix or suffix but keeps the outer space that separates it from
// the metadata.
func cleanAffix(s string) string {
	cleaned := filterVisibleText(s)
	if cleaned == "" {
		return ""
	}
	if strings.TrimLeftFunc(s, unicode.IsSpace) != s {
		cleaned = " " + cleaned
	}
	if strings.TrimRightFunc(s, unicode.IsSpace) != s {
		cleaned += " "
	}
	return cleaned
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
		return filterVisibleText(text)
	}

	return filterVisibleText(extractText(doc))
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

// filterVisibleText keeps printable ASCII and rdsCharset, turning line breaks and tabs
// into spaces.
func filterVisibleText(text string) string {
	visible := strings.Map(func(r rune) rune {
		switch {
		case r >= 32 && r <= 126, strings.ContainsRune(rdsCharset, r):
			return r
		case r == '\n', r == '\r', r == '\t':
			return ' '
		default:
			return -1
		}
	}, toRDSCharset(text))

	return strings.TrimSpace(visible)
}

// toRDSCharset keeps what rdsCharset holds and transliterates the rest per character.
func toRDSCharset(text string) string {
	var b strings.Builder
	for _, r := range norm.NFC.String(text) {
		if r <= 127 || strings.ContainsRune(rdsCharset, r) {
			b.WriteRune(r)
		} else {
			b.WriteString(transliterateToASCII(string(r)))
		}
	}
	return b.String()
}

func transliterateToASCII(text string) string {
	text = multiCharReplacer.Replace(text)

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

func mapNonASCIIToASCII(r rune) rune {
	if r <= 127 {
		return r
	}
	if mapped, ok := nonASCIIToASCII[r]; ok {
		return mapped
	}
	return -1
}
