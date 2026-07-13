// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

func init() {
	pl, err := NewPolicyString("TEST-POLICY", []provider.PolicyStatement{
		{
			Sequence: 10,
			Conditions: []provider.PolicyCondition{
				provider.MatchPrefixSetCondition{
					PrefixSet: &v1alpha1.PrefixSet{
						Spec: v1alpha1.PrefixSetSpec{
							Entries: []v1alpha1.PrefixEntry{
								{
									Prefix: v1alpha1.IPPrefix{
										Prefix: netip.MustParsePrefix("10.0.0.0/8"),
									},
								},
							},
						},
					},
				},
			},
			Actions: v1alpha1.PolicyActions{
				RouteDisposition: v1alpha1.AcceptRoute,
				BgpActions: &v1alpha1.BgpActions{
					SetCommunity: &v1alpha1.SetCommunityAction{
						Communities: []string{"65000:100"},
					},
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	rpl := RoutePolicy{
		Name: "TEST-POLICY",
		Body: pl.String(),
	}
	Register("routepolicy", &rpl)
}

// readGoldenFile reads a golden file from testdata/route_policies
func readGoldenFile(t *testing.T, filename string) string {
	t.Helper()

	path := filepath.Join("testdata", "route_policies", filename)
	content, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read golden file %s", filename)

	return string(content)
}

// ptr returns a pointer to the given value
//
//nolint:modernize // ptr helper is needed for value-returning functions like intstr.FromInt32
func ptr[T any](v T) *T {
	return &v
}

func TestRoutePolicyString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		policyName string
		statements []provider.PolicyStatement
		goldenFile string
		wantErr    bool
	}{
		{
			name:       "simple policy with prefix match and community",
			policyName: "TEST-POLICY",
			statements: []provider.PolicyStatement{
				{
					Sequence: 10,
					Conditions: []provider.PolicyCondition{
						provider.MatchPrefixSetCondition{
							PrefixSet: &v1alpha1.PrefixSet{
								Spec: v1alpha1.PrefixSetSpec{
									Entries: []v1alpha1.PrefixEntry{
										{
											Prefix: v1alpha1.IPPrefix{
												Prefix: netip.MustParsePrefix("10.0.0.0/8"),
											},
										},
									},
								},
							},
						},
					},
					Actions: v1alpha1.PolicyActions{
						RouteDisposition: v1alpha1.AcceptRoute,
						BgpActions: &v1alpha1.BgpActions{
							SetCommunity: &v1alpha1.SetCommunityAction{
								Communities: []string{"65000:100"},
							},
						},
					},
				},
			},
			goldenFile: "simple_policy.txt",
			wantErr:    false,
		},
		{
			name:       "multiple statements with different actions",
			policyName: "MULTI-STATEMENT-POLICY",
			statements: []provider.PolicyStatement{
				{
					Sequence: 10,
					Conditions: []provider.PolicyCondition{
						provider.MatchPrefixSetCondition{
							PrefixSet: &v1alpha1.PrefixSet{
								Spec: v1alpha1.PrefixSetSpec{
									Entries: []v1alpha1.PrefixEntry{
										{
											Prefix: v1alpha1.IPPrefix{
												Prefix: netip.MustParsePrefix("192.168.0.0/16"),
											},
										},
									},
								},
							},
						},
					},
					Actions: v1alpha1.PolicyActions{
						RouteDisposition: v1alpha1.RejectRoute,
					},
				},
				{
					Sequence: 20,
					Conditions: []provider.PolicyCondition{
						provider.MatchPrefixSetCondition{
							PrefixSet: &v1alpha1.PrefixSet{
								Spec: v1alpha1.PrefixSetSpec{
									Entries: []v1alpha1.PrefixEntry{
										{
											Prefix: v1alpha1.IPPrefix{
												Prefix: netip.MustParsePrefix("10.0.0.0/8"),
											},
										},
									},
								},
							},
						},
					},
					Actions: v1alpha1.PolicyActions{
						RouteDisposition: v1alpha1.AcceptRoute,
					},
				},
			},
			goldenFile: "multi_statement_policy.txt",
			wantErr:    false,
		},
		{
			name:       "policy with multiple BGP actions",
			policyName: "BGP-MULTI-ACTION",
			statements: []provider.PolicyStatement{
				{
					Sequence: 10,
					Conditions: []provider.PolicyCondition{
						provider.MatchPrefixSetCondition{
							PrefixSet: &v1alpha1.PrefixSet{
								Spec: v1alpha1.PrefixSetSpec{
									Entries: []v1alpha1.PrefixEntry{
										{
											Prefix: v1alpha1.IPPrefix{
												Prefix: netip.MustParsePrefix("10.0.0.0/8"),
											},
										},
									},
								},
							},
						},
					},
					Actions: v1alpha1.PolicyActions{
						RouteDisposition: v1alpha1.AcceptRoute,
						BgpActions: &v1alpha1.BgpActions{
							SetCommunity: &v1alpha1.SetCommunityAction{
								Communities: []string{"65000:100", "65000:200"},
							},
							SetExtCommunity: &v1alpha1.SetExtCommunityAction{
								Communities: []string{"65000:100"},
							},
						},
					},
				},
			},
			goldenFile: "bgp_multi_action_policy.txt",
			wantErr:    false,
		},
		{
			name:       "policy with all BGP actions combined",
			policyName: "BGP-ALL-ACTIONS",
			statements: []provider.PolicyStatement{
				{
					Sequence: 10,
					Conditions: []provider.PolicyCondition{
						provider.MatchPrefixSetCondition{
							PrefixSet: &v1alpha1.PrefixSet{
								Spec: v1alpha1.PrefixSetSpec{
									Entries: []v1alpha1.PrefixEntry{
										{
											Prefix: v1alpha1.IPPrefix{
												Prefix: netip.MustParsePrefix("172.16.0.0/12"),
											},
										},
									},
								},
							},
						},
					},
					Actions: v1alpha1.PolicyActions{
						RouteDisposition: v1alpha1.AcceptRoute,
						BgpActions: &v1alpha1.BgpActions{
							SetCommunity: &v1alpha1.SetCommunityAction{
								Communities: []string{"65000:100", "65000:200", "65000:300"},
							},
							SetExtCommunity: &v1alpha1.SetExtCommunityAction{
								Communities: []string{"rt:65000:100", "rt:65000:200"},
							},
							SetASPath: &v1alpha1.SetASPathAction{
								Prepend: &v1alpha1.SetASPathPrepend{
									ASNumber: ptr(intstr.FromInt32(65200)), //nolint:modernize // ptr needed for value-returning function
								},
							},
						},
					},
				},
			},
			goldenFile: "bgp_all_actions_policy.txt",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			policy, err := NewPolicyString(tt.policyName, tt.statements)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err, "NewPolicy failed")

			expected := readGoldenFile(t, tt.goldenFile)
			got := policy.String()

			assert.Equal(t, expected, got, "policy configuration mismatch")
		})
	}
}
