# SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

Require kubectl
Require jq

Output examples/demo.gif

Set Shell "bash"
Set Padding 24

Type "# List all devices (drop-in replacement for kubectl get)" Enter Sleep 1s
Type "kubectl net get devices" Enter Sleep 3s

Type "# Filter interfaces by device name" Enter Sleep 1s
Type "kubectl net get interfaces --device leaf1" Enter Sleep 3s

Type "clear" Sleep 1s Enter Sleep 500ms

Type "# Pause a single interface" Enter Sleep 1s
Type "kubectl net pause interfaces lo0" Enter Sleep 3s

Type "# Unpause the interface" Enter Sleep 1s
Type "kubectl net unpause interfaces lo0" Enter Sleep 3s

Type "clear" Sleep 1s Enter Sleep 500ms

Type "# Recursively pause a device and children" Enter Sleep 1s
Type "kubectl net pause devices leaf1 --recursive" Enter Sleep 3s

Type "# Verify the device is paused" Enter Sleep 1s
Type "kubectl net get devices leaf1 -o yaml | yq .spec.paused" Enter Sleep 3s

Type "# Unpause the device and children" Enter Sleep 1s
Type "kubectl net unpause devices leaf1 --recursive" Enter Sleep 3s
