package highlighter

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
)

const AnsiReset = "\x1b[0m"

type HighlightRule struct {
	Name     string
	Pattern  *regexp.Regexp
	ANSI     string
	Priority int
}

type RuleSpec struct {
	Name     string
	Pattern  string
	ANSI     string
	Priority int
}

type HighlightMatch struct {
	Start int
	End   int
	Rule  HighlightRule
}

type Highlighter struct {
	rules []HighlightRule
}

func NewHighlighter(specs []RuleSpec) (*Highlighter, error) {
	rules := make([]HighlightRule, 0, len(specs))
	for _, spec := range specs {
		pattern, err := regexp.Compile(spec.Pattern)
		if err != nil {
			return nil, fmt.Errorf("compile rule %q: %w", spec.Name, err)
		}
		rules = append(rules, HighlightRule{
			Name:     spec.Name,
			Pattern:  pattern,
			ANSI:     spec.ANSI,
			Priority: spec.Priority,
		})
	}
	return &Highlighter{rules: rules}, nil
}

func (h *Highlighter) Highlight(input []byte) []byte {

	if len(input) == 0 {
		return nil
	}

	matches := h.findMatches(input)

	if len(matches) == 0 {
		return append([]byte(nil), input...)
	}

	matches = resolvesOverlaps(matches)

	var output bytes.Buffer
	output.Grow((len(input) + len(matches)*32))

	cursor := 0

	for _, match := range matches {
		if match.Start < cursor {
			continue
		}

		output.Write(input[cursor:match.Start])
		output.WriteString(match.Rule.ANSI)
		output.Write(input[match.Start:match.End])
		output.WriteString(AnsiReset)

		cursor = match.End
	}

	output.Write(input[cursor:])
	return output.Bytes()
}

func (h *Highlighter) findMatches(input []byte) []HighlightMatch {
	var matches []HighlightMatch
	for _, rule := range h.rules {
		locations := rule.Pattern.FindAllIndex(input, -1)
		for _, location := range locations {
			if len(location) != 2 {
				continue
			}

			start := location[0]
			end := location[1]

			if start < 0 || end <= start || end > len(input) {
				continue
			}

			matches = append(matches, HighlightMatch{
				Start: start,
				End:   end,
				Rule:  rule,
			})
		}
	}
	return matches
}

func resolvesOverlaps(matches []HighlightMatch) []HighlightMatch {

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Rule.Priority != matches[j].Rule.Priority {
			return matches[i].Rule.Priority > matches[j].Rule.Priority
		}

		lenI := matches[i].End - matches[i].Start
		lenJ := matches[j].End - matches[j].Start

		if lenI != lenJ {
			return lenI > lenJ
		}

		return matches[i].Start < matches[j].Start
	})

	selected := make([]HighlightMatch, 0, len(matches))
	for _, match := range matches {
		overlaps := false

		for _, selectedMatch := range selected {
			if match.Start < selectedMatch.End && selectedMatch.Start < match.End {
				overlaps = true
				break
			}
		}

		if !overlaps {
			selected = append(selected, match)
		}
	}

	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Start < selected[j].Start
	})
	return selected
}
