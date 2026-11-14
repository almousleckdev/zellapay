package notify

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// sample returns valid data for every template.
func sample(name string) map[string]string {
	all := map[string]map[string]string{
		"invitation": {
			"business": "Boutique Océane", "days": "7", "link": "https://app.example/invite/abc",
		},
		"email_verification": {"hours": "24", "link": "https://app.example/verify/abc"},
		"password_reset":     {"minutes": "30", "link": "https://app.example/reset/abc"},
		"sign_in_alert":      {"when": "14 novembre 2026, 13:00", "device": "Chrome sur macOS"},
		"security_alert":     {"event": "un paiement de 10 000 FCFA a été envoyé"},
	}
	return all[name]
}

func newCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEveryTemplateRendersInBothLanguages(t *testing.T) {
	c := newCatalog(t)
	for name, sp := range specs {
		for _, locale := range []string{LocaleFrench, LocaleEnglish} {
			subject, body, err := c.Render(name, locale, sample(name))
			if err != nil {
				t.Fatalf("%s/%s: %v", name, locale, err)
			}
			label := name + "/" + locale
			checkFinished(t, label, subject, body)
			checkChannelRules(t, label, sp.Channel, [2]string{subject, body})
		}
	}
}

func checkFinished(t *testing.T, label, subject, body string) {
	t.Helper()
	all := subject + body
	empty := strings.TrimSpace(body) == ""
	if empty || strings.Contains(all, "{{") || strings.Contains(all, "<no value>") {
		t.Errorf("%s: unfinished output %q", label, body)
	}
}

// checkChannelRules checks the subject and body, given as [subject, body].
func checkChannelRules(t *testing.T, label string, ch Channel, text [2]string) {
	t.Helper()
	subject, body := text[0], text[1]
	if ch == Email && (subject == "" || strings.ContainsAny(subject, "\r\n")) {
		t.Errorf("%s: bad subject %q", label, subject)
	}
	if n := utf8.RuneCountInString(body); ch == SMS && n > 160 {
		t.Errorf("%s: an SMS must fit in 160 characters, got %d", label, n)
	}
}

func TestEveryDeclaredKeyIsUsedByItsTemplate(t *testing.T) {
	c := newCatalog(t)
	for name, sp := range specs {
		for _, locale := range []string{LocaleFrench, LocaleEnglish} {
			for _, k := range sp.Required {
				data := sample(name)
				data[k] = "UNIQUE-MARKER-" + k
				subject, body, err := c.Render(name, locale, data)
				if err != nil || !strings.Contains(subject+body, "UNIQUE-MARKER-"+k) {
					t.Errorf("%s/%s never uses %q", name, locale, k)
				}
			}
		}
	}
}

func TestRenderRefusesMissingExtraAndUnknown(t *testing.T) {
	c := newCatalog(t)
	missing := sample("invitation")
	delete(missing, "link")
	extra := sample("invitation")
	extra["surprise"] = "x"
	cases := []struct {
		name, template, locale string
		data                   map[string]string
	}{
		{"missing key", "invitation", "fr", missing},
		{"extra key", "invitation", "fr", extra},
		{"unknown template", "nope", "fr", nil},
		{"unknown locale", "invitation", "de", sample("invitation")},
	}
	for _, c2 := range cases {
		_, _, err := c.Render(c2.template, c2.locale, c2.data)
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v", c2.name, err)
		}
	}
}

func TestFrenchTextKeepsItsAccents(t *testing.T) {
	_, body, err := newCatalog(t).Render("invitation", "fr", sample("invitation"))
	if err != nil || !strings.Contains(body, "invité(e)") || !strings.Contains(body, "Océane") {
		t.Fatalf("got %q, %v", body, err)
	}
}
