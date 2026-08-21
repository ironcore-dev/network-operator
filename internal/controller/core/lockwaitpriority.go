// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

// Priority levels for reconcile requests competing for a device lock.
//
// For reference, the full priority hierarchy used across the operator:
//
//	-100 : initial list events at startup (controller-runtime built-in, see
//	       https://github.com/kubernetes-sigs/controller-runtime/blob/main/designs/priorityqueue.md)
//	   0 : default queue priority; used by periodic requeues (RequeueInterval)
//	       of controllers that have already reconciled their resource successfully
//	   1 : LockWaitPriorityDefault — lock-wait requeues for most resource types
//	  10 : LockWaitPriorityHigh    — lock-wait requeues for foundational types
//	                                 (Interface, VRF) whose reconciliation
//	                                 unblocks dependent resources

const (
	// LockWaitPriorityHigh is used by resources that are commonly referenced
	// by other resources. Reconciling them first unblocks dependent resources.
	// Currently applied to: Interface, VRF.
	LockWaitPriorityHigh = 10

	// LockWaitPriorityDefault is used by all other resources competing for a
	// device lock. Higher than the queue default (0) so lock-wait requeues are
	// always served before periodic requeues of already-reconciled resources.
	LockWaitPriorityDefault = 1
)
