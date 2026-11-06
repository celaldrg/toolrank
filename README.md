# toolrank

**Rank MCP `tools/list` tools against a natural-language task — no LLM required.**

`toolrank` is a zero-dependency Go CLI that scores each tool with name / description / schema keyword heuristics, prints the top-N matches, and estimates how many tokens the selected subset would cost. Use it to shrink bloated tool lists before an agent call.

Complements [`mcptax`](https://github.com/celaldrg/mcptax) (cost of the full list) and [`mcpdiff`](https://github.com/celaldrg/mcpdiff) (breaking-change diffs).

## Install

```bash
go install github.com/celaldrg/toolrank@latest
```

Or download a binary from [Releases](https://github.com/celaldrg/toolrank/releases).

```bash
# example (Linux amd64)
curl -sL https://github.com/celaldrg/toolrank/releases/download/v0.1.0/toolrank_v0.1.0_linux_amd64.tar.gz | tar xz
./toolrank -version
```

## Usage

Input is a JSON file that is either a raw MCP `tools/list` result or a bare tool array:

```json
{ "tools": [ { "name": "...", "description": "...", "inputSchema": {} } ] }
```

```bash
toolrank -task "commit changes to git repository" fixtures/tools.json
toolrank -task "commit changes to git repository" -n 5 fixtures/tools.json
toolrank -task "commit changes to git repository" -budget 200 fixtures/tools.json
toolrank -task "search the web" -json fixtures/tools.json
```

### Example

```text
$ toolrank -task "commit changes to git repository" -n 5 fixtures/tools.json
task: commit changes to git repository
mode: top-5  showing=5  tokens=275

#     SCORE   TOKENS   NAME
----  ------  -------  ----
1     16      70       git_commit
      Stage files and create a git commit with a message in the repository.
2     11      49       git_diff
      Show git diff for staged or unstaged changes in the repository.
3     9       50       git_status
      Show the working tree status of a git repository.
4     0       50       browser_click
      Click an element on the current browser page by CSS selector.
5     0       56       browser_navigate
      Open a URL in a headless browser and wait for the page to load.
```

Budget mode keeps selecting score>0 tools until the next one would exceed the token budget:

```text
$ toolrank -task "commit changes to git repository" -budget 200 fixtures/tools.json
task: commit changes to git repository
mode: budget=200  selected=3  tokens=169

#     SCORE   TOKENS   NAME
----  ------  -------  ----
1     16      70       git_commit
      Stage files and create a git commit with a message in the repository.
2     11      49       git_diff
      Show git diff for staged or unstaged changes in the repository.
3     9       50       git_status
      Show the working tree status of a git repository.
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-task` | _(required)_ | Natural-language task to rank against |
| `-n` | `10` | Show top N tools (ignored when `-budget` is set) |
| `-budget` | `0` | Keep selecting top tools until estimated token sum would exceed this (0 = off) |
| `-json` | `false` | Machine-readable JSON output |
| `-version` | | Print version and exit |

## Scoring

1. Tokenize `-task` into lowercase words with length ≥ 3 (punctuation split, duplicates dropped).
2. For each tool, add:
   - **+5** per task token found in the tool **name**
   - **+2** per task token found in the **description**
   - **+3** per task token that matches a schema **property name** (substring either way, property len ≥ 3)
   - **+10** if the full lowercase task is an exact substring of the tool name
3. Stable sort by score descending, then name ascending.
4. Token estimate ≈ `len(JSON(name+description+inputSchema)) / 4`.

No network calls, no embeddings, no API keys — pure stdlib heuristics.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Usage / parse / I/O error |
| `2` | `-budget` mode: no tool with score > 0 fits |

## Develop

```bash
go test ./...
go build -o toolrank .
```

## License

MIT © Celalettin Durğaç
