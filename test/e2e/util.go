package e2e

import "regexp"

func removeSpecialChars(input string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9-]+`)
	return re.ReplaceAllString(input, "")
}
