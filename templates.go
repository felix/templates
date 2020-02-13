package templates

import (
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type Templates struct {
	base      string
	pkg       string
	extension string
	fName     string
	sources   map[string]io.ReadCloser
}

func New(opts ...Option) (*Templates, error) {
	out := &Templates{
		pkg:       "main",
		extension: ".tmpl",
		base:      "./",
		fName:     "loadTemplate",
	}
	for _, o := range opts {
		if err := o(out); err != nil {
			return nil, err
		}
	}
	srcs, err := readTemplates(out.base, "."+out.extension)
	if err != nil {
		return nil, err
	}
	out.sources = srcs
	return out, nil
}

type Option func(*Templates) error

func Base(p string) Option {
	return func(t *Templates) error {
		var err error
		t.base, err = filepath.Abs(p)
		return err
	}
}

func Extension(e string) Option {
	return func(t *Templates) error {
		t.extension = e
		return nil
	}
}
func Package(p string) Option {
	return func(t *Templates) error {
		t.pkg = p
		return nil
	}
}

func readTemplates(root, extension string) (map[string]io.ReadCloser, error) {
	out := make(map[string]io.ReadCloser)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, extension) {
			return nil
		}
		varName := strings.TrimPrefix(path, root)
		varName = strings.TrimSuffix(varName, extension)
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
		data[k] = base64.StdEncoding.EncodeToString(b)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		Package   string
		Base      string
		FName     string
		Templates map[string]string
	}{t.pkg, t.base, t.fName, data}); err != nil {
		return 0, err
	}
	return buf.WriteTo(w)
}

const loader = `package {{ .Package }}

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
)

func {{ .FName }}(n string) ([]byte, error) {
	var templates = map[string]string {
{{ $name, $data := range .Templates }}
		"{{ $name }}": ` + "`" + `{{ $data }}` + "`" + `,
{{ end }}
	}

	d, ok := templates[n]
	if !ok {
		return nil, fmt.Errorf("template not found")
	}
	// Check for overriding file
	b, err := ioutil.ReadFile("{{ .Base }}" + n + "{{ .Extension }}")
	if err == nil && b != nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(d)
}
`
