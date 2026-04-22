# Architecture Sequence Overview

This diagram shows the high-level runtime flow of the Network Operator.
It combines the two main paths in the project:

- `Device` lifecycle reconciliation, including optional provisioning.
- Per-resource configuration reconciliation for objects such as `Interface`, `BGP`, `VRF`, and `Certificate`.

```mermaid
sequenceDiagram
	actor U as User / kubectl
	participant API as Kubernetes API Server
	participant WH as Validating Webhook
	participant MGR as Controller Manager
	participant DEV as DeviceReconciler
	participant CFG as Config Reconciler
	participant BOOT as Provisioning HTTP / TFTP
	participant P as Provider / Transport
	participant DEVICE as Network Device

	U->>API: Apply Device or config CR
	opt Webhook registered for this kind
		API->>WH: Validate request
		WH-->>API: Allow or deny
	end
	API-->>U: Object persisted
	API-)MGR: Watch event

	alt Device reconciliation
		MGR->>+DEV: Reconcile(Device)
		DEV->>API: Read Device and referenced resources
		DEV->>API: Initialize conditions if needed

		alt Provisioning is configured and phase is Pending
			DEV->>API: Set phase=Provisioning and Ready=False
			DEV-->>API: Requeue for follow-up checks
			DEVICE->>+BOOT: Request bootstrap config
			BOOT->>API: Read Device, secrets, and bootstrap assets
			BOOT-->>DEVICE: Return provisioning config, certs, or boot script
			DEVICE->>BOOT: Report provisioning progress
			BOOT->>API: Update provisioning status
			API-)MGR: Watch event for Device update
		end

		DEV->>+P: Connect to provider
		P->>DEVICE: Read device facts and ports
		DEVICE-->>P: Inventory and operational state
		P-->>-DEV: Device details
		DEV->>API: Patch status, labels, and Ready condition
		DEV-->>-MGR: Reconcile complete

	else Config reconciliation
		MGR->>+CFG: Reconcile(config CR)
		CFG->>API: Read resource and resolve spec.deviceRef
		CFG->>API: Read target Device
		CFG->>CFG: Check paused state
		CFG->>CFG: Acquire per-device lock
		CFG->>API: Ensure finalizer, device label, and owner reference
		CFG->>+P: Connect using device endpoint and credentials
		P->>DEVICE: Ensure intended configuration
		DEVICE-->>P: Apply result
		P-->>-CFG: Success or error
		CFG->>API: Patch Ready condition and status
		CFG->>CFG: Release per-device lock
		CFG-->>-MGR: Reconcile complete
	end

	Note over DEV,CFG: Each config CR reconciles independently, but writes to the same device are serialized by a per-device lock.
```

## Reading Guide

- `Device` is the central resource. Other configuration resources target a device through `spec.deviceRef`.
- The admission webhook is optional and only runs for resource kinds that register validation webhooks.
- Provisioning and inline TFTP are optional manager-hosted services used during device bootstrap.
- After bootstrap, reconcilers connect through the provider and transport layer to push or verify device state.
- Status and conditions are always written back to Kubernetes so the API remains the source of truth.

## Device Provisioning Flow

This diagram focuses only on the `Device` lifecycle, including optional provisioning, progress checks, and the transition into the running state.

```mermaid
sequenceDiagram
	actor U as User / kubectl
	participant API as Kubernetes API Server
	participant MGR as Controller Manager
	participant DEV as DeviceReconciler
	participant BOOT as Provisioning HTTP / TFTP
	participant P as Provider / Transport
	participant DEVICE as Network Device

	U->>API: Apply Device CR
	API-->>U: Object persisted
	API-)MGR: Watch event

	MGR->>+DEV: Reconcile(Device)
	DEV->>API: Read Device and status
	DEV->>API: Initialize conditions if needed

	alt Provisioning configured and phase is Pending
		DEV->>API: Set phase=Provisioning and Ready=False
		DEV-->>API: Requeue for follow-up checks
		DEVICE->>+BOOT: Request bootstrap config
		BOOT->>API: Read Device, secrets, and bootstrap assets
		BOOT-->>DEVICE: Return provisioning config, certs, or boot script
		DEVICE->>BOOT: Report provisioning progress
		BOOT->>API: Update provisioning status
		API-)MGR: Watch event for Device update
		DEV->>API: Requeue until provisioning completes
	end

	DEV->>+P: Connect to provider
	P->>DEVICE: Read device facts and ports
	DEVICE-->>P: Inventory and operational state
	P-->>-DEV: Device details
	DEV->>API: Patch status, labels, and Ready condition
	DEV-->>-MGR: Reconcile complete
```

## Config Reconcile Flow

This diagram focuses on per-resource reconciliation for configuration CRs such as `Interface`, `BGP`, `VRF`, and `Certificate`.

```mermaid
sequenceDiagram
	actor U as User / kubectl
	participant API as Kubernetes API Server
	participant WH as Validating Webhook
	participant MGR as Controller Manager
	participant CFG as Config Reconciler
	participant P as Provider / Transport
	participant DEVICE as Network Device

	U->>API: Apply config CR
	opt Webhook registered for this kind
		API->>WH: Validate request
		WH-->>API: Allow or deny
	end
	API-->>U: Object persisted
	API-)MGR: Watch event

	MGR->>+CFG: Reconcile(config CR)
	CFG->>API: Read resource and resolve spec.deviceRef
	CFG->>API: Read target Device
	CFG->>CFG: Check paused state
	CFG->>CFG: Acquire per-device lock
	CFG->>API: Ensure finalizer, device label, and owner reference
	CFG->>+P: Connect using device endpoint and credentials
	P->>DEVICE: Ensure intended configuration
	DEVICE-->>P: Apply result
	P-->>-CFG: Success or error
	CFG->>API: Patch Ready condition and status
	CFG->>CFG: Release per-device lock
	CFG-->>-MGR: Reconcile complete

	Note over CFG: Writes targeting the same device are serialized by the per-device lock.
```
