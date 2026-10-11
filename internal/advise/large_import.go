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
			Path:  it.From,
			Saves: tokens,
		}
		if diff, ok := replaceImportDiff(it.From, it.Ref); ok {
			f.Diff = diff
		}
		out = append(out, f)
	}
	return out
}

// replaceImportDiff proposes turning "@ref" into "`ref` (read it when relevant)" on the line
// where it appears.
func replaceImportDiff(file, ref string) (string, bool) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "@"+ref) {
			fixed := strings.Replace(line, "@"+ref, "`"+ref+"` (read it when relevant)", 1)
			return "- " + line + "\n+ " + fixed, true
		}
	}
	return "", false
}
