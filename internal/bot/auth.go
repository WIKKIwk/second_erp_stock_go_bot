package bot

import (
	"fmt"
	"strings"
	"unicode"
)

func supplierNameMatches(expected, actual string) bool {
	left := normalizeComparableText(expected)
	right := normalizeComparableText(actual)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}

	distance := levenshteinDistance(left, right)
	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}
	if maxLen <= 4 {
		return distance <= 1
	}
	return distance <= 2
}

func normalizeComparableText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validateStrongPassword(value string) error {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 8 {
		return fmt.Errorf("Parol kamida 8 belgidan iborat bo'lishi kerak")
	}

	hasLetter := false
	hasDigit := false
	for _, r := range trimmed {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return fmt.Errorf("Parol harf va son aralash bo'lishi kerak")
	}
	return nil
}

func levenshteinDistance(left, right string) int {
	if left == right {
		return 0
	}
	if len(left) == 0 {
		return len(right)
	}
	if len(right) == 0 {
		return len(left)
	}

	prev := make([]int, len(right)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(left); i++ {
		current := make([]int, len(right)+1)
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 0
			if left[i-1] != right[j-1] {
				cost = 1
			}

			insertCost := current[j-1] + 1
			deleteCost := prev[j] + 1
			replaceCost := prev[j-1] + cost
			current[j] = minInt(insertCost, deleteCost, replaceCost)
		}
		prev = current
	}

	return prev[len(right)]
}

func minInt(values ...int) int {
	best := values[0]
	for _, value := range values[1:] {
		if value < best {
			best = value
		}
	}
	return best
}
