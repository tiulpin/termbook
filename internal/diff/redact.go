package diff

import (
	"fmt"
	"regexp"

	"github.com/tiulpin/termbook/internal/config"
)

type Redactor struct {
	rules []rule
}

type rule struct {
	re      *regexp.Regexp
	replace []byte
}

func NewRedactor(in []config.RedactRule) (*Redactor, error) {
	out := make([]rule, 0, len(in))
	for i, r := range in {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("redact rule %d (%q): %w", i, r.Pattern, err)
		}
		out = append(out, rule{re: re, replace: []byte(r.Replace)})
	}
	return &Redactor{rules: out}, nil
}

func (r *Redactor) Apply(data []byte) []byte {
	if r == nil || len(r.rules) == 0 {
		return data
	}
	out := data
	for _, ru := range r.rules {
		out = ru.re.ReplaceAll(out, ru.replace)
	}
	return out
}
