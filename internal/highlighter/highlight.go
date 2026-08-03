package highlighter

import (
	"bytes"
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

type HighlightMatch struct {
	Start int
	End   int
	Rule  HighlightRule
}

type Highlighter struct {
	rules []HighlightRule
}

func NewHighlighter() *Highlighter {
	return &Highlighter{
		rules: []HighlightRule{
			{
				Name:     "success",
				Pattern:  regexp.MustCompile(`(?i)(up|established|success|ok|done|completed|finished):?`),
				ANSI:     "\x1b[1;32m",
				Priority: 110,
			},
			{
				Name:     "error",
				Pattern:  regexp.MustCompile(`(?i)(down|error|fail|failed|exception|panic|fatal):?`),
				ANSI:     "\x1b[1;31m",
				Priority: 100,
			},
			{
				Name:     "warning",
				Pattern:  regexp.MustCompile(`(?i)(warning|warn|alert|notice):?`),
				ANSI:     "\x1b[1;33m",
				Priority: 90,
			},
			{
				Name:     "ipv4",
				Pattern:  regexp.MustCompile(`(?i)([0-9]{1,3}\.){3}[0-9]{1,3}`),
				ANSI:     "\x1b[1;34m",
				Priority: 80,
			},
			{
				Name:     "asn",
				Pattern:  regexp.MustCompile(`(?i)\bAS[0-9]{1,10}\b`),
				ANSI:     "\x1b[38;2;95;170;255m",
				Priority: 80,
			},
			{
				Name:     "huawei-interface",
				Pattern:  regexp.MustCompile(`\b(100GE|40GE|25GE|10GE|GigabitEthernet|Eth-Trunk|Vlanif|LoopBack|Tunnel|NULL|Nve|Virtual-Template)[A-Za-z0-9/_.:-]*\b`),
				ANSI:     "\x1b[38;2;95;255;255m",
				Priority: 70,
			},
		},
	}
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
