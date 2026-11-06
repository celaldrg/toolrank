package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("toolrank", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	task := fs.String("task", "", "natural-language task to rank tools against (required)")
	n := fs.Int("n", 10, "show top N tools")
	budget := fs.Int("budget", 0, "keep selecting top tools until estimated tokens would exceed budget (0=disabled)")
	jsonOut := fs.Bool("json", false, "machine-readable JSON output")
	showVersion := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *showVersion {
		fmt.Printf("toolrank %s\n", version)
		return 0
	}

	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "usage: toolrank -task \"...\" [-n 10] [-budget 2000] [-json] <tools.json>")
		return 1
	}
	if strings.TrimSpace(*task) == "" {
		fmt.Fprintln(os.Stderr, "error: -task is required")
		return 1
	}

	data, err := os.ReadFile(rest[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", rest[0], err)
		return 1
	}

	tools, err := ParseTools(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	ranked := RankTools(tools, *task)

	var selected []RankedTool
	budgetMode := *budget > 0
	if budgetMode {
		selected = SelectByBudget(ranked, *budget)
		if len(selected) == 0 {
			fmt.Fprintln(os.Stderr, "error: no tools with score > 0 fit within budget")
			return 2
		}
	} else {
		selected = SelectTop(ranked, *n)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		type out struct {
			Task           string       `json:"task"`
			Budget         int          `json:"budget,omitempty"`
			SelectedTokens int          `json:"selected_tokens"`
			Tools          []RankedTool `json:"tools"`
		}
		payload := out{
			Task:   *task,
			Tools:  selected,
		}
		if budgetMode {
			payload.Budget = *budget
		}
		for _, t := range selected {
			payload.SelectedTokens += t.Tokens
		}
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintf(os.Stderr, "error: encode json: %v\n", err)
			return 1
		}
		return 0
	}

	totalTok := 0
	for _, t := range selected {
		totalTok += t.Tokens
	}

	fmt.Printf("task: %s\n", *task)
	if budgetMode {
		fmt.Printf("mode: budget=%d  selected=%d  tokens=%d\n\n", *budget, len(selected), totalTok)
	} else {
		fmt.Printf("mode: top-%d  showing=%d  tokens=%d\n\n", *n, len(selected), totalTok)
	}

	fmt.Printf("%-4s  %-6s  %-7s  %s\n", "#", "SCORE", "TOKENS", "NAME")
	fmt.Printf("%-4s  %-6s  %-7s  %s\n", "----", "------", "-------", "----")
	for i, t := range selected {
		fmt.Printf("%-4d  %-6d  %-7d  %s\n", i+1, t.Score, t.Tokens, t.Name)
		if desc := strings.TrimSpace(t.Description); desc != "" {
			if len(desc) > 90 {
				desc = desc[:87] + "..."
			}
			fmt.Printf("      %s\n", desc)
		}
	}
	return 0
}
