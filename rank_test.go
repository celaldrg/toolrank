package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestParseToolsObject(t *testing.T) {
	raw := []byte(`{"tools":[{"name":"a","description":"alpha"},{"name":"b","description":"beta"}]}`)
	tools, err := ParseTools(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 || tools[0].Name != "a" || tools[1].Name != "b" {
		t.Fatalf("unexpected: %+v", tools)
	}
}

func TestParseToolsArray(t *testing.T) {
	raw := []byte(`[{"name":"x","description":"xray"}]`)
	tools, err := ParseTools(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "x" {
		t.Fatalf("unexpected: %+v", tools)
	}
}

func TestParseToolsEmpty(t *testing.T) {
	if _, err := ParseTools([]byte("")); err == nil {
		t.Fatal("expected error for empty input")
	}
	if _, err := ParseTools([]byte("   ")); err == nil {
		t.Fatal("expected error for whitespace input")
	}
}

func TestParseToolsMissingArray(t *testing.T) {
	if _, err := ParseTools([]byte(`{"foo":1}`)); err == nil {
		t.Fatal("expected error for missing tools")
	}
}

func TestTokenizeTask(t *testing.T) {
	toks := TokenizeTask("Read a file from the filesystem and git commit")
	got := map[string]bool{}
	for _, tok := range toks {
		got[tok] = true
		if len(tok) < 3 {
			t.Fatalf("short token: %q", tok)
		}
	}
	for _, want := range []string{"read", "file", "from", "the", "filesystem", "and", "git", "commit"} {
		if !got[want] {
			t.Fatalf("missing token %q in %v", want, toks)
		}
	}
	toks2 := TokenizeTask("git git GIT")
	if len(toks2) != 1 || toks2[0] != "git" {
		t.Fatalf("dedupe failed: %v", toks2)
	}
}

func TestScoreToolWeights(t *testing.T) {
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]string{"type": "string"},
		},
	})
	tool := Tool{
		Name:        "read_file",
		Description: "Read contents of a file from disk",
		InputSchema: schema,
	}
	tokens := TokenizeTask("read file path from disk")
	score := ScoreTool(tool, "read file path from disk", tokens)
	if score < 15 {
		t.Fatalf("expected strong score, got %d", score)
	}

	exact := ScoreTool(Tool{Name: "read file path from disk"}, "read file path from disk", tokens)
	plain := ScoreTool(Tool{Name: "other"}, "read file path from disk", tokens)
	if exact <= plain {
		t.Fatalf("exact name substring should boost: exact=%d plain=%d", exact, plain)
	}
}

func TestRankToolsStableSort(t *testing.T) {
	tools := []Tool{
		{Name: "zeta", Description: "nothing useful here"},
		{Name: "alpha_git", Description: "git operations"},
		{Name: "beta_git", Description: "git operations"},
		{Name: "gamma", Description: "unrelated"},
	}
	ranked := RankTools(tools, "git commit")
	for i := 1; i < len(ranked); i++ {
		if ranked[i-1].Score < ranked[i].Score {
			t.Fatal("not sorted by score desc")
		}
		if ranked[i-1].Score == ranked[i].Score && ranked[i-1].Name > ranked[i].Name {
			t.Fatalf("name order at equal score: %s > %s", ranked[i-1].Name, ranked[i].Name)
		}
	}
	var gitOnes []string
	for _, r := range ranked {
		if r.Name == "alpha_git" || r.Name == "beta_git" {
			gitOnes = append(gitOnes, r.Name)
		}
	}
	if len(gitOnes) != 2 {
		t.Fatalf("expected both git tools: %+v", ranked)
	}
}

func TestSelectByBudget(t *testing.T) {
	ranked := []RankedTool{
		{Tool: Tool{Name: "a"}, Score: 10, Tokens: 100},
		{Tool: Tool{Name: "b"}, Score: 8, Tokens: 100},
		{Tool: Tool{Name: "c"}, Score: 5, Tokens: 50},
		{Tool: Tool{Name: "d"}, Score: 0, Tokens: 10},
	}
	sel := SelectByBudget(ranked, 200)
	if len(sel) != 2 || sel[0].Name != "a" || sel[1].Name != "b" {
		t.Fatalf("unexpected selection: %+v", sel)
	}
	sel2 := SelectByBudget(ranked, 150)
	if len(sel2) != 2 || sel2[0].Name != "a" || sel2[1].Name != "c" {
		t.Fatalf("expected a+c for budget 150: %+v", sel2)
	}
	sel3 := SelectByBudget(ranked, 5)
	if len(sel3) != 0 {
		t.Fatalf("expected empty for tiny budget: %+v", sel3)
	}
}

func TestSelectTop(t *testing.T) {
	ranked := []RankedTool{
		{Tool: Tool{Name: "a"}, Score: 3},
		{Tool: Tool{Name: "b"}, Score: 2},
		{Tool: Tool{Name: "c"}, Score: 1},
	}
	if got := SelectTop(ranked, 2); len(got) != 2 || got[1].Name != "b" {
		t.Fatalf("top 2: %+v", got)
	}
	if got := SelectTop(ranked, 0); len(got) != 3 {
		t.Fatalf("n=0 should return all: %+v", got)
	}
	if got := SelectTop(ranked, 99); len(got) != 3 {
		t.Fatalf("n>len should return all: %+v", got)
	}
}

func TestEstimateTokens(t *testing.T) {
	schema, _ := json.Marshal(map[string]any{
		"type":       "object",
		"properties": map[string]any{"path": map[string]string{"type": "string"}},
	})
	tok := EstimateTokens(Tool{Name: "read_file", Description: "Read a file", InputSchema: schema})
	if tok < 1 {
		t.Fatalf("expected positive tokens, got %d", tok)
	}
}

func TestFixturesRanking(t *testing.T) {
	data, err := os.ReadFile("fixtures/tools.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	tools, err := ParseTools(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) < 8 {
		t.Fatalf("expected >=8 fixtures, got %d", len(tools))
	}
	ranked := RankTools(tools, "commit changes to git repository")
	if ranked[0].Score <= 0 {
		t.Fatal("top tool should score > 0 for git task")
	}
	foundGit := false
	for i, r := range ranked {
		if i >= 3 {
			break
		}
		if strings.Contains(strings.ToLower(r.Name), "git") ||
			strings.Contains(strings.ToLower(r.Description), "git") {
			foundGit = true
		}
	}
	if !foundGit {
		names := make([]string, 0, 3)
		for i, r := range ranked {
			if i >= 3 {
				break
			}
			names = append(names, r.Name)
		}
		t.Fatalf("expected a git-related tool in top 3: %v", names)
	}
}
