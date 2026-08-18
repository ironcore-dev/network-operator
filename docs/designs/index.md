# Design Documents

This directory contains design documents for the Network Operator project.
They record architectural decisions and serve as guardrails for new
development.

Not every change requires a design document — only those that introduce
new abstractions, change existing behavior in a non-trivial way, or
establish patterns that future contributions must follow. Bug fixes,
new brick implementations following existing patterns, and additive
API fields do not need a design doc.

## Process

1. Copy `NNNN-template.md` and assign the next number.
2. Fill in the sections. Keep it concise.
3. Submit as a pull request for review.
4. Once merged with status `Accepted`, the decision is binding.

To supersede a design document, create a new one referencing the
original and update the original's status to `Superseded by NNNN`.

## Index

| # | Title | Status |
|---|-------|--------|
| [0001](./0001-device-lifecycle.md) | Device Lifecycle Phases | Accepted |
| [0002](./0002-provider-interface.md) | Provider Interface | Accepted |
| [0003](./0003-status-conditions.md) | Status Conditions | Accepted |
| [0004](./0004-device-resource-locking.md) | Device Resource Locking | Accepted |
| [0005](./0005-brick-architecture.md) | Brick Architecture | Accepted |
| [0006](./0006-api-deprecation-strategy.md) | API Deprecation Strategy | Accepted |
