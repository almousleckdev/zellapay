package notify

import (
	"embed"
	"fmt"
	"slices"
	"strings"
	"text/template"
)

//go:embed templates
var templateFS embed.FS

// spec declares a template: its channel and the data keys it needs. A
// message must supply exactly these keys, which catches typos and forgotten
// values (a verification email with no link) before anything is sent.
type spec struct {
	Channel  Channel
	Required []string
}

var specs = map[string]spec{
	"invitation":         {Email, []string{"business", "days", "link"}},
	"email_verification": {Email, []string{"hours", "link"}},
	"password_reset":     {Email, []string{"link", "minutes"}},
	"sign_in_alert":      {Email, []string{"device", "when"}},
	"security_alert":     {SMS, []string{"event"}},
}

type key struct{ name, locale string }

type compiled struct {
	subject *template.Template // nil for SMS
	body    *template.Template
}

// Catalog renders the message templates.
type Catalog struct {
	byKey map[key]compiled
}

// NewCatalog parses every embedded template and checks that each declared
// template exists in both languages.
func NewCatalog() (*Catalog, error) {
	c := &Catalog{byKey: map[key]compiled{}}
	for name, sp := range specs {
		for _, locale := range []string{LocaleFrench, LocaleEnglish} {
			raw, err := templateFS.ReadFile("templates/" + locale + "/" + name + ".txt")
			if err != nil {
				return nil, fmt.Errorf("template %s (%s): %w", name, locale, err)
			}
			t, err := compile(name, sp.Channel, string(raw))
			if err != nil {
				return nil, fmt.Errorf("template %s (%s): %w", name, locale, err)
			}
			c.byKey[key{name, locale}] = t
		}
	}
	return c, nil
}

func compile(name string, ch Channel, raw string) (compiled, error) {
	subject, body := "", strings.TrimSpace(raw)
	if ch == Email {
		head, rest, ok := strings.Cut(raw, "\n\n")
		line, found := strings.CutPrefix(head, "Subject: ")
		if !ok || !found || strings.Contains(line, "\n") {
			return compiled{}, fmt.Errorf("an email template starts with a one-line Subject")
		}
		subject, body = line, strings.TrimSpace(rest)
	}
	out := compiled{}
	var err error
	if ch == Email {
		if out.subject, err = parse(name+".subject", subject); err != nil {
			return compiled{}, err
		}
	}
	out.body, err = parse(name, body)
	return out, err
}

func parse(name, text string) (*template.Template, error) {
	return template.New(name).Option("missingkey=error").Parse(text)
}

// Channel returns the channel a template is for.
func (c *Catalog) Channel(name string) (Channel, bool) {
	sp, ok := specs[name]
	return sp.Channel, ok
}

// Render fills a template. The data must have exactly the declared keys.
func (c *Catalog) Render(
	name, locale string, data map[string]string,
) (subject, body string, err error) {
	sp, ok := specs[name]
	if !ok {
		return "", "", fmt.Errorf("%w: unknown template %q", ErrInvalidInput, name)
	}
	t, ok := c.byKey[key{name, locale}]
	if !ok {
		return "", "", fmt.Errorf("%w: unsupported locale %q", ErrInvalidInput, locale)
	}
	if err := checkKeys(sp.Required, data); err != nil {
		return "", "", err
	}
	if t.subject != nil {
		if subject, err = execute(t.subject, data); err != nil {
			return "", "", err
		}
	}
	body, err = execute(t.body, data)
	return subject, body, err
}

func execute(t *template.Template, data map[string]string) (string, error) {
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", fmt.Errorf("%w: render: %w", ErrInvalidInput, err)
	}
	return b.String(), nil
}

func checkKeys(required []string, data map[string]string) error {
	got := make([]string, 0, len(data))
	for k := range data {
		got = append(got, k)
	}
	slices.Sort(got)
	want := slices.Clone(required)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return fmt.Errorf("%w: data must have exactly %v, got %v", ErrInvalidInput, want, got)
	}
	return nil
}
