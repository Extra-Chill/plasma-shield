// Package rules provides the rule engine for filtering traffic.
package rules

import (
	"regexp"
	"strings"
)

// CompiledRule holds a rule with its pre-compiled matchers.
type CompiledRule struct {
	Rule           *Rule
	CommandMatcher *regexp.Regexp // Compiled command pattern (nil if no pattern)
	DomainMatcher  *regexp.Regexp // Compiled domain pattern (nil if no domain)
}

// CompileRule compiles patterns in a rule for efficient matching.
// Command patterns use glob syntax (* matches anything).
// Domain patterns support exact match and wildcard prefix (*.example.com).
func CompileRule(r *Rule) (*CompiledRule, error) {
	cr := &CompiledRule{Rule: r}

	// Compile command pattern if present
	if r.Pattern != "" {
		regex, err := globToRegex(r.Pattern)
		if err != nil {
			return nil, err
		}
		cr.CommandMatcher = regex
	}

	// Compile domain pattern if present
	if r.Domain != "" {
		regex, err := domainToRegex(r.Domain)
		if err != nil {
			return nil, err
		}
		cr.DomainMatcher = regex
	}

	return cr, nil
}

// MatchCommand checks if a command matches this rule's pattern.
func (cr *CompiledRule) MatchCommand(cmd string) bool {
	if cr.CommandMatcher == nil {
		return false
	}
	return cr.CommandMatcher.MatchString(cmd)
}

// MatchDomain checks if a domain matches this rule's domain pattern.
func (cr *CompiledRule) MatchDomain(domain string) bool {
	if cr.DomainMatcher == nil {
		return false
	}
	// Normalize domain to lowercase for matching
	return cr.DomainMatcher.MatchString(strings.ToLower(domain))
}

// globToRegex converts a glob pattern to a regex.
// * matches any sequence of characters (non-greedy).
// All other regex special chars are escaped.
func globToRegex(pattern string) (*regexp.Regexp, error) {
	// Escape regex special characters except *
	var b strings.Builder
	b.WriteString("(?i)") // Case-insensitive
	
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			b.WriteString(".*?") // Non-greedy match
		case '.', '+', '?', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}

	return regexp.Compile(b.String())
}

// domainToRegex converts a domain pattern to a regex.
// Supports:
// - Exact match: "example.com"
// - Wildcard prefix: "*.example.com" (matches any subdomain)
// - Contains wildcard: "*xmr*" (matches if domain contains "xmr")
func domainToRegex(pattern string) (*regexp.Regexp, error) {
	pattern = strings.ToLower(pattern)
	
	var b strings.Builder
	b.WriteString("(?i)^") // Case-insensitive, anchor start
	
	// Check for different wildcard patterns
	if strings.HasPrefix(pattern, "*.") {
		// Wildcard subdomain: *.example.com matches sub.example.com or example.com
		suffix := pattern[2:] // Remove "*."
		escaped := escapeRegexDomain(suffix)
		b.WriteString("([a-z0-9-]+\\.)*") // Optional subdomain parts
		b.WriteString(escaped)
	} else if strings.Contains(pattern, "*") {
		// Contains wildcards - convert to regex
		for i := 0; i < len(pattern); i++ {
			c := pattern[i]
			switch c {
			case '*':
				b.WriteString(".*")
			case '.':
				b.WriteString("\\.")
			default:
				b.WriteByte(c)
			}
		}
	} else {
		// Exact match
		b.WriteString(escapeRegexDomain(pattern))
	}
	
	b.WriteString("$") // Anchor end

	return regexp.Compile(b.String())
}

// escapeRegexDomain escapes special regex characters in a domain string.
func escapeRegexDomain(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '.', '+', '?', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
