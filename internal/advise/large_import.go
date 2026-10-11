package advise

import (
	"fmt"
	"os"
	"strings"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
)

// largeImportTokens is the size above which an always-loaded import is worth replacing.
const largeImportTokens = 500

// largeImport flags big files pulled into every session through "@path" and proposes a plain
// reference, so Claude reads the file only when it needs it.
func largeImport(in Input) []Finding {
	var out []Finding
	for _, it := range in.Items {
		if it.Kind != "import" || it.Load != claudecode.Always {
			continue
		}
		tokens := analytics.EstimateTokens(it.Text)
		if tokens < largeImportTokens {
			continue
		}
		f := Finding{
			Title: fmt.Sprintf("@%s adds ≈ %d tokens to every session", it.Ref, tokens),
			Detail: "The whole file is loaded at session start even when the task does not need it. " +
				"Refer to it in plain text and say when to read it; Claude opens it on demand.",
			Path:   it.From,
			Saves:  tokens,
			Params: map[string]string{"ref": it.Ref, "tokens": fmt.Sprint(tokens)},
		}
		if line, fixed, n, ok := replaceImport(it.From, it.Ref); ok {
			f.Diff = hunk(it.From, n) + "- " + line + "\n+ " + fixed
			f.Edits = []Edit{{Path: it.From, Old: line, New: fixed}}
		}
		out = append(out, f)
	}
	return out
}

// replaceImport finds the line with "@ref" and proposes the same line with
// "`ref` (read it when relevant)" instead; n is the line's number.
func replaceImport(file, ref string) (line, fixed string, n int, ok bool) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", "", 0, false
	}
	for i, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "@"+ref) {
			return line, strings.Replace(line, "@"+ref, "`"+ref+"` (read it when relevant)", 1), i + 1, true
		}
	}
	return "", "", 0, false
}
