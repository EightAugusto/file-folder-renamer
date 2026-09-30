package pattern

import (
	"fmt"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

func Trace(spec Spec, input string, kind rename.NodeKind) (TraceResult, error) {
	return traceWithBuilder(spec, input, kind, Build)
}

func traceWithBuilder(spec Spec, input string, kind rename.NodeKind, build func(Spec) (Pattern, error)) (TraceResult, error) {
	if kind == "" {
		kind = rename.NodeKindFile
	}
	if kind != rename.NodeKindFile && kind != rename.NodeKindFolder {
		return TraceResult{}, fmt.Errorf("unknown node kind %q", kind)
	}
	definition, err := build(spec)
	if err != nil {
		return TraceResult{}, err
	}

	parts := rename.Parse(input)
	result := TraceResult{Input: input, Kind: kind, Stem: parts.Stem, Protected: parts.Protected}
	current := input
	if kind == rename.NodeKindFile {
		current = parts.Stem
		result.Extension = rename.NormalizeExtension(parts.Extension)
	}
	if !parts.Protected {
		for index, rule := range definition.Rules {
			before := current
			after, applyErr := rule.Apply(before)
			step := TraceStep{Index: index, Kind: rule.Kind(), Input: before, Output: after}
			if index < len(spec.Rules) {
				step.Description = spec.Rules[index].Description
			}
			if applyErr != nil {
				step.Error = applyErr.Error()
				result.Steps = append(result.Steps, step)
				return result, applyErr
			}
			result.Steps = append(result.Steps, step)
			current = after
		}
	}
	if kind == rename.NodeKindFile {
		result.Output = current + result.Extension
	} else {
		result.Output = current
	}
	return result, nil
}

func RunTests(spec Spec) []TestResult {
	results := make([]TestResult, 0, len(spec.Tests))
	for _, testCase := range spec.Tests {
		trace, err := Trace(spec, testCase.Input, testCase.Kind)
		result := TestResult{Case: testCase, Actual: trace.Output}
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Passed = trace.Output == testCase.Expected
		}
		results = append(results, result)
	}
	return results
}
