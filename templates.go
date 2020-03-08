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
	mappings   []Mapping
	base       string
	pkg        string
	extensions []string
	prefix     string
	htmlTmpl   bool
	textTmpl   bool
}

type Mapping struct {
	Base       string
	Source     string
	Extensions []string
	sources    map[string][]byte
}

type logger func(...interface{})

var log logger = func(a ...interface{}) {}

func New(opts ...Option) (*Templates, error) {
	out := &Templates{
		pkg:        "main",
		extensions: nil,
		mappings:   []Mapping{{Base: ".", Source: "templates", Extensions: nil}},
		prefix:     "load",
	}
	for _, o := range opts {
		if err := o(out); err != nil {
			return nil, err
		}
	}
	for i, m := range out.mappings {
		log("reading templates from", m.Source, "with extensions", m.Extensions)
		if err := readTemplates(out.base, &m); err != nil {
			return nil, err
		}
		out.mappings[i] = m
	}
	return out, nil
}

func Must(t *Templates, err error) *Templates {
	if err != nil {
		panic(err)
	}
	return t
}

func validSuffix(path string, extensions []string) bool {
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

func readTemplates(base string, m *Mapping) error {
	m.sources = make(map[string][]byte)
	if m.Base != "" {
		base = m.Base
	}
	src := filepath.Join(base, m.Source)
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log("failed", err)
			return err
		}
		if info.IsDir() {
			return nil
		}

		log("considering", path)
		log("root", m.Source)
		varName, _ := filepath.Rel(src, path)
		log("varName", varName)
		if !validSuffix(path, m.Extensions) {
			return nil
		}

		m.sources[varName], err = ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		log("found template", varName)
		return nil
	})
}

func (t *Templates) WriteTo(w io.Writer) (int64, error) {
	tmpl, err := template.New("loader").Parse(loader)
	if err != nil {
		return 0, err
	}
	data := make(map[string]string)
	for _, m := range t.mappings {
		for k, b := range m.sources {
			data[k] = base64.RawURLEncoding.EncodeToString(b)
		}
	}
	var buf bytes.Buffer
	vars := map[string]interface{}{
		"package":   t.pkg,
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

type Templater interface {
	ExecuteTemplate(io.Writer, string, interface{}) error
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
