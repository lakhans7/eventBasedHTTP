package ai

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

var jsonObjectRE = regexp.MustCompile(`(?s)\{.*\}`)

// tryParseExtraction best-effort parses the JSON object the model was asked
// to return for requirement extraction. A malformed response degrades to a
// nil extraction (the raw draft is still shown to the seller) rather than
// erroring the whole request — the model is not a trusted parser.
func tryParseExtraction(text string) *domain.RequirementExtraction {
	match := jsonObjectRE.FindString(text)
	if match == "" {
		return nil
	}
	var ex domain.RequirementExtraction
	if err := json.Unmarshal([]byte(match), &ex); err != nil {
		return nil
	}
	return &ex
}

var sentimentLineRE = regexp.MustCompile(`(?i)SENTIMENT:\s*(positive|neutral|negative)`)

func extractSentiment(text string) string {
	m := sentimentLineRE.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.ToLower(m[1])
}
