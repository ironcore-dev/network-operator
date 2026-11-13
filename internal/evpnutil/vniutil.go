package evpnutil

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	evpnv1alpha1 "github.com/ironcore-dev/network-operator/api/evpn/v1alpha1"
	poolv1alpha1 "github.com/ironcore-dev/network-operator/api/pool/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
)

// ComputeVNIRouteTarget computes the VNI and RFC 8365 auto-derived route target for a given claim and fabric.
func ComputeVNIRouteTarget[T conditions.Setter](owner T, fabric *evpnv1alpha1.Fabric, claim *poolv1alpha1.Claim) (vni uint64, rt string, err error) {
	asNumber := fabric.Spec.Overlay.IBGP.ASNumber.String()
	// RFC 8365 auto-derivation requires a 2-byte ASN; bit size 16 rejects larger and dotted ASNs.
	asn, err := strconv.ParseUint(asNumber, 10, 16)
	if err != nil || asn == 0 {
		conditions.Set(owner, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.UnsupportedASNumberReason,
			Message: fmt.Sprintf("Fabric %q has unsupported AS number %q, want 1..65535", fabric.Name, asNumber),
		})
		return 0, "", reconcile.TerminalError(fmt.Errorf("fabric %q has unsupported AS number %q, want 1..65535", fabric.Name, asNumber))
	}

	vni, err = strconv.ParseUint(claim.Status.Value, 10, 24)
	if err != nil || vni == 0 || vni > 16777214 {
		conditions.Set(owner, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  evpnv1alpha1.InvalidVNIReason,
			Message: fmt.Sprintf("Claim %q has invalid VNI %q", claim.Name, claim.Status.Value),
		})
		return 0, "", reconcile.TerminalError(fmt.Errorf("claim %q has invalid VNI %q", claim.Name, claim.Status.Value))
	}

	// RFC 8365 divides the 32-bit local administrator as follows:
	//
	//	bit 31       bits 30..28  bits 27..24  bits 23..0
	//	+------------+------------+------------+----------------+
	//	| A          | Type       | Domain ID  | Service ID     |
	//	| 1 bit      | 3 bits     | 4 bits     | 24 bits        |
	//	+------------+------------+------------+----------------+
	//
	// A=0 means auto-derived, Type=1 means VXLAN, Domain ID=0, and Service ID is the VNI.
	localAdministrator := uint32(1<<28) | uint32(vni)

	// Represent the RFC 8365 auto-derived route target in ASN:local-administrator notation.
	rt = fmt.Sprintf("%d:%d", asn, localAdministrator)

	return vni, rt, nil
}
