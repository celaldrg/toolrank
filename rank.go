package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Tool is one MCP tools/list entry.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// RankedTool is a tool with its relevance score and token estimate.
type RankedTool struct {
	Tool
	Score  int `json:"score"`
	Tokens int `json:"tokens"`
}

// ParseTools accepts either {"tools":[...]} or a bare tool array.
func ParseTools(data []byte) ([]Tool, error) {
	data = bytesTrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	if data[0] == '[' {
		var tools []Tool
		if err := json.Unmarshal(data, &tools); err != nil {
			return nil, fmt.Errorf("parse tools array: %w", err)
		}
		return tools, nil
	}

	var wrap struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("parse tools object: %w", err)
	}
	if wrap.Tools == nil {
		return nil, fmt.Errorf("missing tools array")
	}
	return wrap.Tools, nil
}

func bytesTrimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && unicode.IsSpace(rune(b[i])) {
		i++
	}
	for j > i && unicode.IsSpace(rune(b[j-1])) {
		j--
	}
	return b[i:j]
}

// TokenizeTask splits a task into lowercase words of length >= 3.
func TokenizeTask(task string) []string {
	fields := strings.FieldsFunc(strings.ToLower(task), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	seen := make(map[string]struct{})
	for _, f := range fields {
		if len(f) < 3 {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

// schemaPropertyNames extracts property names from a JSON Schema object.
func schemaPropertyNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}
	names := make([]string, 0, len(schema.Properties))
	for k := range schema.Properties {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ScoreTool scores a tool against task tokens.
// Weights: name match=5, description match=2, schema property match=3.
// Exact substring of full lowercase task in name adds +10.
func ScoreTool(tool Tool, task string, tokens []string) int {
	nameLower := strings.ToLower(tool.Name)
	descLower := strings.ToLower(tool.Description)
	props := schemaPropertyNames(tool.InputSchema)
	propLower := make([]string, len(props))
	for i, p := range props {
		propLower[i] = strings.ToLower(p)
	}

	score := 0
	taskLower := strings.ToLower(strings.TrimSpace(task))
	if taskLower != "" && strings.Contains(nameLower, taskLower) {
		score += 10
	}

	for _, tok := range tokens {
		if strings.Contains(nameLower, tok) {
			score += 5
		}
		if strings.Contains(descLower, tok) {
			score += 2
		}
		for _, p := range propLower {
			if strings.Contains(p, tok) || strings.Contains(tok, p) && len(p) >= 3 {
				score += 3
				break
			}
		}
	}
	return score
}

// EstimateTokens roughly estimates tokens as chars/4 of the tool definition JSON.
func EstimateTokens(tool Tool) int {
	type slim struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	}
	b, err := json.Marshal(slim{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: tool.InputSchema,
	})
	if err != nil {
		return 0
	}
	n := len(b) / 4
	if n < 1 && len(b) > 0 {
		n = 1
	}
	return n
}

// RankTools scores and stably sorts tools by score desc, then name asc.
func RankTools(tools []Tool, task string) []RankedTool {
	tokens := TokenizeTask(task)
	ranked := make([]RankedTool, 0, len(tools))
	for _, t := range tools {
		s := ScoreTool(t, task, tokens)
		ranked = append(ranked, RankedTool{
			Tool:   t,
			Score:  s,
			Tokens: EstimateTokens(t),
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].Name < ranked[j].Name
	})
	return ranked
}

// SelectTop returns the first n ranked tools (or all if n <= 0 or n > len).
func SelectTop(ranked []RankedTool, n int) []RankedTool {
	if n <= 0 || n > len(ranked) {
		return ranked
	}
	return ranked[:n]
}

// SelectByBudget keeps selecting top-scoring tools while cumulative tokens <= budget.
// Only tools with score > 0 are considered.
func SelectByBudget(ranked []RankedTool, budget int) []RankedTool {
	var selected []RankedTool
	sum := 0
	for _, r := range ranked {
		if r.Score <= 0 {
			continue
		}
		if sum+r.Tokens > budget {
			continue
		}
		selected = append(selected, r)
		sum += r.Tokens
	}
	return selected
}
