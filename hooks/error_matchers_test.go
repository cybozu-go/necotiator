package hooks

import (
	"errors"
	"fmt"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HaveStatusErrorMessage(m types.GomegaMatcher) types.GomegaMatcher {
	return WithTransform(func(e error) (string, error) {
		statusErr := &apierrors.StatusError{}
		if !errors.As(e, &statusErr) {
			return "", fmt.Errorf("HaveStatusErrorMessage expects a *errors.StatusError, but got %T", e)
		}
		return statusErr.ErrStatus.Message, nil
	}, m)
}

func HaveStatusErrorReason(m types.GomegaMatcher) types.GomegaMatcher {
	return WithTransform(func(e error) (metav1.StatusReason, error) {
		statusErr := &apierrors.StatusError{}
		if !errors.As(e, &statusErr) {
			return "", fmt.Errorf("HaveStatusErrorReason expects a *errors.StatusError, but got %T", e)
		}
		return statusErr.ErrStatus.Reason, nil
	}, m)
}
