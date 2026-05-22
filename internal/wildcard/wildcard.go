package wildcard

import (
	"regexp"
	"strings"
)

// Regexp converts a simple '*'/'?' wildcard pattern into an anchored regexp.
func Regexp(pattern string) string {
	var builder strings.Builder
	builder.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			builder.WriteString(".*")
		case '?':
			builder.WriteString(".")
		default:
			builder.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	builder.WriteString("$")
	return builder.String()
}

// Compile validates and compiles a wildcard pattern.
func Compile(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(Regexp(pattern))
}

// Match reports whether value matches a simple '*'/'?' wildcard pattern.
func Match(pattern, value string) bool {
	matched, err := regexp.MatchString(Regexp(pattern), value)
	return err == nil && matched
}
