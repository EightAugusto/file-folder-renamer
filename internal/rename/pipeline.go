package rename

import "github.com/eightaugusto/file-folder-renamer/internal/rules"

type Pipeline struct {
	name  string
	rules []rules.Rule
}

func New(name string, rr []rules.Rule) *Pipeline {
	cloned := make([]rules.Rule, len(rr))
	for index, rule := range rr {
		cloned[index] = rules.Clone(rule)
	}
	return &Pipeline{name: name, rules: cloned}
}

func (p *Pipeline) Name() string {
	if p == nil {
		return ""
	}
	return p.name
}

func (p *Pipeline) Apply(input string) (string, error) {
	if p == nil {
		return input, nil
	}
	out := input
	for _, rule := range p.rules {
		transformed, err := rule.Apply(out)
		if err != nil {
			return "", err
		}
		out = transformed
	}
	return out, nil
}
