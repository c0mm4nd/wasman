package wat

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/c0mm4nd/wasman"
	"github.com/c0mm4nd/wasman/config"
)

// TestCompileSpectestCorpus sweeps every plain (module ...) form in the
// spectest .wast sources: whatever the checker accepts must either compile
// into a binary the real decoder+validator accepts, or be rejected as an
// explicitly unsupported form — never miscompile
func TestCompileSpectestCorpus(t *testing.T) {
	root := filepath.Join("..", "spectest", "testdata")
	if _, err := os.Stat(root); err != nil {
		t.Skip("spectest testdata not generated")
	}

	var valid, compiled, unsupported int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".wast") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		toks, err := lex(src)
		if err != nil {
			return nil // scripts with post-MVP tokens etc.
		}
		forest, err := parseSExprs(toks)
		if err != nil {
			return nil
		}

		for i := range forest {
			top := &forest[i]
			if top.head() != "module" {
				continue
			}
			fields := moduleFields(top)
			// skip (module binary "...") / (module quote "...") payloads
			if len(fields) > 0 && !fields[0].isList() && fields[0].atom.kind == tokKeyword {
				continue
			}
			if checkModuleFields(fields) != nil {
				continue // not a checker-valid module (post-MVP etc.)
			}
			valid++

			// re-render the module form for Compile's public entry
			sub := src[:0:0]
			sub = append(sub, renderNode(top)...)
			bin, err := Compile(sub)
			if err != nil {
				if strings.Contains(err.Error(), "unsupported") {
					unsupported++
					continue
				}
				t.Errorf("%s: module #%d: compile failed on a valid module: %v", path, i, err)
				continue
			}

			if _, err := wasman.NewModule(config.ModuleConfig{}, bytes.NewReader(bin)); err != nil {
				t.Errorf("%s: module #%d: compiled binary rejected by the validator: %v", path, i, err)
				continue
			}
			compiled++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("corpus: %d valid text modules, %d compiled+validated, %d unsupported forms", valid, compiled, unsupported)
	if compiled < 100 {
		t.Fatalf("corpus sweep compiled only %d modules — harness looks broken", compiled)
	}
}

// renderNode turns a parsed s-expression back into text
func renderNode(n *node) string {
	if !n.isList() {
		t := n.atom
		switch {
		case t.kind == tokString:
			return `"` + t.text + `"` // text is the raw source without quotes
		case t.kind == tokID && t.str != nil:
			return `$"` + t.text[1:] + `"` // string-form id
		}
		return t.text
	}
	parts := make([]string, len(n.list))
	for i := range n.list {
		parts[i] = renderNode(&n.list[i])
	}
	return "(" + strings.Join(parts, " ") + ")"
}
