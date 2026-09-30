package rules

type Rule interface {
	Kind() Kind
	Apply(input string) (string, error)
}
