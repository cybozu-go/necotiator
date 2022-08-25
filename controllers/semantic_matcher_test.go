package controllers

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

func SemanticEqual(expected interface{}) types.GomegaMatcher {
	return &semanticMatcher{
		expected: expected,
		compare:  equality.Semantic.DeepEqual,
	}
}

func SemanticDerivative(expected interface{}) types.GomegaMatcher {
	return &semanticMatcher{
		expected: expected,
		compare:  equality.Semantic.DeepDerivative,
	}
}

type semanticMatcher struct {
	expected interface{}
	compare  func(a1, a2 interface{}) bool
}

func (matcher *semanticMatcher) Match(actual interface{}) (bool, error) {
	return matcher.compare(matcher.expected, actual), nil
}

var diffOptions = []cmp.Option{
	cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	}),
	cmp.Comparer(func(a, b metav1.MicroTime) bool {
		return a.UTC() == b.UTC()
	}),
	cmp.Comparer(func(a, b metav1.Time) bool {
		return a.UTC() == b.UTC()
	}),
	cmp.Comparer(func(a, b labels.Selector) bool {
		return a.String() == b.String()
	}),
	cmp.Comparer(func(a, b fields.Selector) bool {
		return a.String() == b.String()
	}),
}

func (matcher *semanticMatcher) FailureMessage(actual interface{}) (message string) {
	diff := cmp.Diff(matcher.expected, actual, diffOptions...)
	return fmt.Sprintf("diff: \n%s", diff)
}

func (matcher *semanticMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	diff := cmp.Diff(matcher.expected, actual, diffOptions...)
	return fmt.Sprintf("diff: \n%s", diff)
}
