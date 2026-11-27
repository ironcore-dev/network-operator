// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ctx = context.Background()

func TestReferenceNamespaces(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reference Namespace Validation Suite")
}

// +kubebuilder:scaffold:imports
// +kubebuilder:scaffold:scheme
// +kubebuilder:scaffold:webhook
