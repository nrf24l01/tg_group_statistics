package update

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"gorm.io/datatypes"
)

func tokenizeWords(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}

	words := make([]string, 0, 16)
	var b strings.Builder
	b.Grow(len(text))

	flush := func() {
		if b.Len() == 0 {
			return
		}
		words = append(words, b.String())
		b.Reset()
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	if len(words) == 0 {
		return nil
	}
	return words
}

func countWords(text string) map[string]int64 {
	tokens := tokenizeWords(text)
	if len(tokens) == 0 {
		return map[string]int64{}
	}
	result := make(map[string]int64, len(tokens))
	addWordCounts(result, tokens)
	return result
}

func addWordCounts(dst map[string]int64, tokens []string) {
	if len(tokens) == 0 {
		return
	}
	for _, t := range tokens {
		if t == "" {
			continue
		}
		dst[t]++
	}
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

func ensureWordDayMap(m map[string]map[string]int64, dateKey string) map[string]int64 {
	if m == nil {
		return nil
	}
	day, ok := m[dateKey]
	if !ok || day == nil {
		day = make(map[string]int64)
		m[dateKey] = day
	}
	return day
}

func wordCountsToJSONMap(counts map[string]int64) datatypes.JSONMap {
	return int64CountsToJSONMap(counts)
}

func int64CountsToJSONMap(src map[string]int64) datatypes.JSONMap {
	if len(src) == 0 {
		return datatypes.JSONMap{}
	}
	out := make(datatypes.JSONMap, len(src))
	for k, v := range src {
		if k == "" || v == 0 {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return datatypes.JSONMap{}
	}
	return out
}

func jsonMapToWordCounts(jm datatypes.JSONMap) map[string]int64 {
	return jsonMapToInt64Counts(jm)
}

func jsonMapToInt64Counts(src datatypes.JSONMap) map[string]int64 {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]int64, len(src))
	for k, v := range src {
		if k == "" || v == nil {
			continue
		}
		switch vv := v.(type) {
		case int:
			out[k] = int64(vv)
		case int32:
			out[k] = int64(vv)
		case int64:
			out[k] = vv
		case uint:
			out[k] = int64(vv)
		case uint32:
			out[k] = int64(vv)
		case uint64:
			out[k] = int64(vv)
		case float32:
			out[k] = int64(vv)
		case float64:
			out[k] = int64(vv)
		case json.Number:
			if n, err := vv.Int64(); err == nil {
				out[k] = n
			}
		case string:
			if n, err := strconv.ParseInt(vv, 10, 64); err == nil {
				out[k] = n
			}
		default:
			// ignore unknown types
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
