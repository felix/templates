package templates

type Option func(*Templates) error

func Map(m []Mapping) Option {
	return func(t *Templates) error {
		t.mappings = m
		return nil
	}
}

func Base(s string) Option {
	return func(t *Templates) error {
		t.base = s
		return nil
	}
}

func Debug(f func(...interface{})) Option {
	return func(t *Templates) error {
		log = f
		log("debugging templates")
		return nil
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
