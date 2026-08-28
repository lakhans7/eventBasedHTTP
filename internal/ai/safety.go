package ai

import (
	"regexp"
	"strings"
)

// RiskFlag names a category the safety pipeline can raise. Flags are always
// surfaced to the seller in the API response — they never silently change
// model behavior the seller can't see (docs/security.md).
const (
	RiskPromptInjection     = "prompt_injection_attempt"
	RiskCredentialRequest   = "credential_or_secret_request"
	RiskUnrealisticDeadline = "unrealistic_deadline_request"
	RiskDiscountRequest     = "discount_or_refund_request"
	RiskOffPlatformPayment  = "off_platform_payment_request"
	RiskPersonalInfo        = "personal_information_request"
)

// injectionPatterns catches the classic "ignore your instructions" family of
// attacks (docs/security.md section 12). This is a heuristic first line of
// defense; the real defense is that buyer content is always passed to the
// LLM as clearly-delimited untrusted data (see pipeline.go), never
// concatenated into the system prompt.
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore (all |any |your )?(previous|prior|above) instructions`),
	regexp.MustCompile(`(?i)disregard (the )?(above|previous|prior)`),
	regexp.MustCompile(`(?i)you are now`),
	regexp.MustCompile(`(?i)reveal (your |the )?(system prompt|instructions)`),
	regexp.MustCompile(`(?i)what (is|are) your (system prompt|instructions)`),
	regexp.MustCompile(`(?i)new instructions:`),
	regexp.MustCompile(`(?i)pretend (you|to) (are|be)`),
}

var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(password|api[ -]?key|secret[ -]?key|access[ -]?token|credit card|ssn|social security)\b`),
}

var unrealisticDeadlinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(today|tonight|in (the )?next (hour|30 minutes|hour)|within (an|1) hour|right now|asap.*(hour|minute))\b`),
}

var discountPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(discount|refund|cheaper|lower (the )?price|free of charge|for free)\b`),
}

var offPlatformPaymentPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(paypal|venmo|zelle|wire transfer|pay you directly|outside (of )?fiverr|off[- ]platform)\b`),
}

var personalInfoPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(home address|phone number|whatsapp number|personal email)\b`),
}

// DetectRisks scans untrusted buyer text and returns every category that matched.
// It never modifies the text — flags are informational, surfaced to the seller.
func DetectRisks(buyerText string) []string {
	var flags []string
	add := func(patterns []*regexp.Regexp, flag string) {
		for _, p := range patterns {
			if p.MatchString(buyerText) {
				flags = append(flags, flag)
				return
			}
		}
	}
	add(injectionPatterns, RiskPromptInjection)
	add(credentialPatterns, RiskCredentialRequest)
	add(unrealisticDeadlinePatterns, RiskUnrealisticDeadline)
	add(discountPatterns, RiskDiscountRequest)
	add(offPlatformPaymentPatterns, RiskOffPlatformPayment)
	add(personalInfoPatterns, RiskPersonalInfo)
	return flags
}

// WrapUntrusted delimits buyer-supplied text so the prompt construction step
// can tell the model, unambiguously, that this block is data to respond to
// and never instructions to follow. See docs/security.md "instruction
// precedence".
func WrapUntrusted(buyerText string) string {
	// Neutralize any attempt to close the delimiter early from within buyer text.
	escaped := strings.ReplaceAll(buyerText, "</buyer_message>", "[buyer_message tag removed]")
	return "<buyer_message untrusted=\"true\">\n" + escaped + "\n</buyer_message>"
}

// ValidateResponse re-checks a generated draft against seller-defined hard
// constraints after generation (docs/security.md step 7). It returns
// additional risk flags rather than blocking outright — the human reviewer
// always makes the final call.
func ValidateResponse(draft string, minProjectUSD int, restrictions string) []string {
	var flags []string
	lower := strings.ToLower(draft)

	if strings.Contains(lower, "discount") || strings.Contains(lower, "% off") {
		flags = append(flags, "draft_mentions_discount_review_against_policy")
	}
	if restrictions != "" {
		for _, word := range strings.Fields(strings.ToLower(restrictions)) {
			if len(word) > 4 && strings.Contains(lower, word) {
				// Weak heuristic on purpose: this only flags for human attention,
				// it never silently rewrites the draft.
				flags = append(flags, "draft_may_touch_seller_restriction")
				break
			}
		}
	}
	for _, p := range credentialPatterns {
		if p.MatchString(draft) {
			flags = append(flags, "draft_requests_credentials")
			break
		}
	}
	return flags
}
