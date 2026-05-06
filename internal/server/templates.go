package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*.html.tmpl
var embeddedTemplates embed.FS

// templates wraps a parsed *template.Template plus a list of names.
type templates struct {
	t *template.Template
}

func loadTemplates(overrideDir string) (*templates, error) {
	funcs := template.FuncMap{
		"formatBytes":  humanBytes,
		"formatNumber": formatNumber,
		"jsonStr":      jsonStr,
		"typeLabel":    TypeChineseLabel,
		"defaultStr": func(s, def string) string {
			if s == "" {
				return def
			}
			return s
		},
	}

	root := template.New("").Funcs(funcs)

	if overrideDir != "" {
		if _, err := os.Stat(overrideDir); err == nil {
			matches, err := filepath.Glob(filepath.Join(overrideDir, "*.html.tmpl"))
			if err != nil {
				return nil, err
			}
			if len(matches) > 0 {
				t, err := root.ParseFiles(matches...)
				if err != nil {
					return nil, err
				}
				return &templates{t: t}, nil
			}
		}
	}

	t, err := root.ParseFS(embeddedTemplates, "templates/*.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse embedded templates: %w", err)
	}
	return &templates{t: t}, nil
}

func (ts *templates) render(w io.Writer, name string, data any) error {
	return ts.t.ExecuteTemplate(w, name, data)
}

// humanBytes mirrors index.php's formatLibraryBytes(): integer for bytes,
// 2 decimals once we move past KB.
func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	idx := 0
	for v >= 1024 && idx < len(units)-1 {
		v /= 1024
		idx++
	}
	return fmt.Sprintf("%.2f %s", v, units[idx])
}

// formatNumber mirrors PHP's number_format($n) — thousands separators.
func formatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var out strings.Builder
	first := len(s) % 3
	if first > 0 {
		out.WriteString(s[:first])
		if len(s) > first {
			out.WriteByte(',')
		}
	}
	for i := first; i < len(s); i += 3 {
		out.WriteString(s[i : i+3])
		if i+3 < len(s) {
			out.WriteByte(',')
		}
	}
	if neg {
		return "-" + out.String()
	}
	return out.String()
}

// jsonStr returns a JSON-encoded scalar safe for inlining inside <script>.
// Returned as template.JS so html/template doesn't double-escape it.
func jsonStr(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return template.JS(`""`)
	}
	return template.JS(b)
}
