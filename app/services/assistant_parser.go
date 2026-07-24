package services

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxTopicLength = 100

func NormalizeForSearch(value string) string {
	value = strings.ReplaceAll(value, "Đ", "D")
	value = strings.ReplaceAll(value, "đ", "d")
	value = norm.NFD.String(strings.ToLower(value))

	var normalized strings.Builder
	normalized.Grow(len(value))

	for _, character := range value {
		switch {
		case unicode.Is(unicode.Mn, character):
			continue
		case unicode.IsLetter(character), unicode.IsDigit(character):
			normalized.WriteRune(character)
		default:
			normalized.WriteByte(' ')
		}
	}

	return strings.Join(strings.Fields(normalized.String()), " ")
}

func ExtractTopicCandidate(question string) string {
	normalizedQuestion := NormalizeForSearch(question)
	if !hasStatisticsIntent(normalizedQuestion) {
		return ""
	}

	if quoted := extractQuoted(question); quoted != "" {
		return limitTopic(NormalizeForSearch(quoted))
	}

	for _, marker := range []string{"noi ve chu de ", "thuoc chu de ", "chu de ", "noi ve ", "ve "} {
		if index := strings.LastIndex(normalizedQuestion, marker); index >= 0 {
			return limitTopic(strings.TrimSpace(normalizedQuestion[index+len(marker):]))
		}
	}

	return ""
}

func hasStatisticsIntent(question string) bool {
	return strings.Contains(question, "bao nhieu") ||
		strings.Contains(question, "co may") ||
		strings.Contains(question, "dem ")
}

func extractQuoted(value string) string {
	start := strings.IndexAny(value, "\"“")
	if start < 0 {
		return ""
	}

	rest := value[start+1:]
	end := strings.IndexAny(rest, "\"”")
	if end < 0 {
		return ""
	}

	return strings.TrimSpace(rest[:end])
}

func limitTopic(topic string) string {
	runes := []rune(topic)
	if len(runes) > maxTopicLength {
		runes = runes[:maxTopicLength]
	}

	return strings.TrimSpace(string(runes))
}
