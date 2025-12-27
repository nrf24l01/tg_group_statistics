package update

import (
	"strings"
	"unicode"

	"gorm.io/datatypes"
)

func countWords(text string) map[string]int64 {
	text = strings.ToLower(text)

	result := make(map[string]int64)
	var b strings.Builder
	b.Grow(len(text))

	flush := func() {
		if b.Len() == 0 {
			return
		}
		w := b.String()
		b.Reset()
		if w == "" {
			return
		}
		result[w]++
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	return result
}

func mergeWordCounts(dst map[string]int64, add map[string]int64) map[string]int64 {
	if len(add) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]int64, len(add))
	}
	for w, c := range add {
		dst[w] += c
	}
	return dst
}

func wordCountsToJSONMap(counts map[string]int64) datatypes.JSONMap {
	jm := datatypes.JSONMap{}
	for w, c := range counts {
		jm[w] = c
	}
	return jm
}

func jsonMapToWordCounts(jm datatypes.JSONMap) map[string]int64 {
	if len(jm) == 0 {
		return nil
	}
	result := make(map[string]int64, len(jm))
	for k, v := range jm {
		switch n := v.(type) {
		case int:
			result[k] = int64(n)
		case int32:
			result[k] = int64(n)
		case int64:
			result[k] = n
		case uint:
			result[k] = int64(n)
		case uint32:
			result[k] = int64(n)
		case uint64:
			result[k] = int64(n)
		case float32:
			result[k] = int64(n)
		case float64:
			result[k] = int64(n)
		case string:
			// ignore unexpected string values
		default:
			// ignore unknown types
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
