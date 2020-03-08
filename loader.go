package templates

const loader = `package {{ .package }}

// This is a generated file.

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

var {{ .prefix }}Overrider func(string) string

func {{ .prefix }}Template(n string) ([]byte, error) {
	var templates = map[string]string {
{{- range $name, $data := .templates }}
		"{{ $name }}": ` + "`" + `{{ $data }}` + "`" + `,
{{- end }}
	}

	d, ok := templates[n]
	if !ok {
		return nil, fmt.Errorf("template %q not found", n)
	}
	if {{ .prefix }}Overrider != nil {
		n = {{ .prefix }}Overrider(n)
		// Check for overriding file
		b, err := ioutil.ReadFile(n)
		if err == nil && b != nil {
			fmt.Printf("templates: overriding %q\n", n)
			return b, nil
		}
	}
	data, err := base64.RawURLEncoding.DecodeString(d)
	if err != nil {
		return nil, fmt.Errorf("failed to load template %q: %w", n, err)
	}
	return data, nil
}

{{- if .textTmpl }}

func {{ .prefix }}TextTemplate(names []string, funcs text.FuncMap) (*text.Template, error) {
	var out *html.Template
	for _, n := range names {
		data, err := {{ .prefix }}Template(n)
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

// {{ .prefix }}HTMLTemplate loads all templates listed separately.
func {{ .prefix }}HTMLTemplate(names []string, funcs html.FuncMap) (*html.Template, error) {
	var out *html.Template
	for _, n := range names {
		data, err := {{ .prefix }}Template(n)
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

// {{ .prefix }}HTMLTemplateMap loads all templates listed grouped into named templates.
func {{ .prefix }}HTMLTemplateMap(tmplMap map[string][]string, funcs html.FuncMap) (map[string]*html.Template, error) {
	out := make(map[string]*html.Template)
	for n, tmpls := range tmplMap {
		tmp := html.New(n).Funcs(funcs)
		for _, t := range tmpls {
			data, err := {{ .prefix }}Template(t)
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
