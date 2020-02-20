package templates

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Templates struct {
	base       string
	pkg        string
	extensions []string
	prefix     string
	htmlTmpl   bool
	textTmpl   bool
	sources    map[string]io.ReadCloser
}

func New(opts ...Option) (*Templates, error) {
	out := &Templates{
		pkg:        "main",
		extensions: nil,
		base:       "./",
		prefix:     "",
	}
	for _, o := range opts {
		if err := o(out); err != nil {
			return nil, err
		}
	}
	srcs, err := readTemplates(out.base, out.extensions)
	if err != nil {
		return nil, err
	}
	out.sources = srcs
	return out, nil
}

func Must(t *Templates, err error) *Templates {
	if err != nil {
		panic(err)
	}
	return t
}

type Option func(*Templates) error

func Base(p string) Option {
	return func(t *Templates) error {
		var err error
		t.base, err = filepath.Abs(p)
		return err
	}
}

func Extensions(e []string) Option {
	return func(t *Templates) error {
		t.extensions = e
		return nil
	}
}

func Package(p string) Option {
	return func(t *Templates) error {
		t.pkg = p
		return nil
	}
}

func EnableHTMLTemplates() Option {
	return func(t *Templates) error {
		t.htmlTmpl = true
		return nil
	}
}

func EnableTextTemplates() Option {
	return func(t *Templates) error {
		t.textTmpl = true
		return nil
	}
}

func FunctionPrefix(p string) Option {
	return func(t *Templates) error {
		t.prefix = p
		return nil
	}
}

func hasSuffix(path string, extensions []string) bool {
	if extensions == nil {
		return true
	}
	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func readTemplates(root string, extensions []string) (map[string]io.ReadCloser, error) {
	out := make(map[string]io.ReadCloser)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		varName := strings.TrimPrefix(path, root)
		if !hasSuffix(path, extensions) {
			return nil
		}
		rc, err := os.Open(path)
		if err != nil {
			return err
		}
		out[varName] = rc
		return nil
	})
	return out, nil
}

func (t *Templates) WriteTo(w io.Writer) (int64, error) {
	tmpl, err := template.New("loader").Parse(loader)
	if err != nil {
		return 0, err
	}
	data := make(map[string]string, len(t.sources))
	for k, rc := range t.sources {
		b, err := ioutil.ReadAll(rc)
		if err != nil {
			return 0, err
		}
		data[k] = base64.RawURLEncoding.EncodeToString(b)
	}
	var buf bytes.Buffer
	vars := map[string]interface{}{
		"package":   t.pkg,
		"base":      t.base,
		"prefix":    t.prefix,
		"templates": data,
		"textTmpl":  t.textTmpl,
		"htmlTmpl":  t.htmlTmpl,
	}
	if err := tmpl.Execute(&buf, vars); err != nil {
		return 0, err
	}
	return buf.WriteTo(w)
}

type Renderer func(http.ResponseWriter, string, interface{}) error

func (r Renderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	return r(w, name, data)
}

func NewRenderer(tmpls map[string]*template.Template) Renderer {
	var bufPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}

	return func(w http.ResponseWriter, name string, data interface{}) error {
		tmpl, ok := tmpls[name]
		if !ok {
			return fmt.Errorf("missing template %s", name)
		}
		//fmt.Println("rendering", name)
		//fmt.Println(tmpl.DefinedTemplates())

		buf := bufPool.Get().(*bytes.Buffer)
		defer bufPool.Put(buf)

		if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
			return err
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		buf.WriteTo(w)
		return nil
	}
}

const loader = `package {{ .package }}

import (
	"encoding/base64"
	"fmt"
{{- if .htmlTmpl }}
	html "html/template"
{{- end }}
	"io/ioutil"
{{- if .textTmpl }}
	text "text/template"
{{- end }}
)

func {{ .prefix }}LoadTemplate(n string) ([]byte, error) {
	var templates = map[string]string {
{{- range $name, $data := .templates }}
		"{{ $name }}": ` + "`" + `{{ $data }}` + "`" + `,
{{- end }}
	}

	d, ok := templates[n]
	if !ok {
		return nil, fmt.Errorf("template %q not found", n)
	}
	// Check for overriding file
	b, err := ioutil.ReadFile("{{ .base }}" + n)
	if err == nil && b != nil {
		return b, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(d)
	if err != nil {
		return nil, fmt.Errorf("failed to load template %q: %w", n, err)
	}
	return data, nil
}

func {{ .prefix }}MustLoadTemplate(n string) []byte {
	b, err := {{ .prefix }}LoadTemplate(n)
	if err != nil {
		panic(err)
	}
	return b
}
{{- if .textTmpl }}

func {{ .prefix }}LoadTextTemplate(names []string, funcs text.FuncMap) (*text.Template, error) {
	var out *html.Template
	for _, n := range names {
		data, err := {{ .prefix }}LoadTemplate(n)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = html.New(n).Funcs(funcs)
		}
		out.Parse(string(data))
	}
	return out, nil
}
{{- end }}
{{- if .htmlTmpl }}

// {{ .prefix }}LoadHTMLTemplate loads all templates listed separately.
func {{ .prefix }}LoadHTMLTemplate(names []string, funcs html.FuncMap) (*html.Template, error) {
	var out *html.Template
	for _, n := range names {
		data, err := {{ .prefix }}LoadTemplate(n)
		if err != nil {
			return nil, err
		}
		if out == nil {
			// The first one is used for the name
			out = html.New(n).Funcs(funcs)
		}
		out.Parse(string(data))
	}
	return out, nil
}

// {{ .prefix }}LoadHTMLTemplateMap loads all templates listed grouped into named templates.
func {{ .prefix }}LoadHTMLTemplateMap(tmplMap map[string][]string, funcs html.FuncMap) (map[string]*html.Template, error) {
	out := make(map[string]*html.Template)
	for n, tmpls := range tmplMap {
		tmp := html.New(n).Funcs(funcs)
		for _, t := range tmpls {
			data, err := {{ .prefix }}LoadTemplate(t)
			if err != nil {
				return nil, err
			}
			tmp.Parse(string(data))
		}
		out[n] = tmp
	}
	return out, nil
}
{{- end }}
`
