package ansi

import "regexp"

type CompiledRule struct {
	Description string
	Regex       *regexp.Regexp
	Style       string
	Exclusive   bool
}
