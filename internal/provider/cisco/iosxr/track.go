// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"fmt"
	"strings"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

// MaxTracks is the maximum number of tracks configurable on an IOS-XR device (1-2048).
const MaxTracks = 2028

// Tracks represents a collection of track entries on an IOS-XR device.
// It implements the sort.Interface to allow sorting by the RTR value of the track type.
type Tracks struct {
	Track []Track `json:"track"`
}

func (t Tracks) Len() int           { return len(t.Track) }
func (t Tracks) Swap(i, j int)      { t.Track[i], t.Track[j] = t.Track[j], t.Track[i] }
func (t Tracks) Less(i, j int) bool { return t.Track[i].Type.RTR < t.Track[j].Type.RTR }

// StaticRouteTrackID returns the track-name for a single nexthop of a static
// route. Track names follow the pattern
// vrfname-<prefix-with-underscores>-<nexthop-with-underscores>, e.g.
// management-192_168_1_0-10_8_1_1.
func StaticRouteTrackID(vrf, addr, nextHop string) string {
	return fmt.Sprintf("%s-%s-%s",
		vrf,
		strings.ReplaceAll(addr, ".", "_"),
		strings.ReplaceAll(nextHop, ".", "_"))
}

// GarbageCollect returns the tracks, their IPSLA schedule entries and matching
// IPSLA operations for the given track-names, ready to be passed to
// SetBuilder.Delete
func (t *Tracks) GarbageCollect(trackIDs ...string) []gnmiext.DataElement {
	want := make(map[string]struct{}, len(trackIDs))
	for _, id := range trackIDs {
		want[id] = struct{}{}
	}
	var del []gnmiext.DataElement
	for i := range t.Track {
		if _, ok := want[t.Track[i].TrackID]; !ok {
			continue
		}
		rtr := t.Track[i].Type.RTR
		del = append(del,
			&t.Track[i],
			&IPSLASchedule{OperationNumber: rtr},
			&IPSLAOperation{OperationNumber: rtr},
		)
	}
	return del
}

func (t *Tracks) GetTrackByID(trackID string) *Track {
	for i := range t.Track {
		if t.Track[i].TrackID == trackID {
			return &t.Track[i]
		}
	}
	return nil
}

func (t *Tracks) XPath() string {
	return "Cisco-IOS-XR-um-track-cfg:tracks"
}

func (t *Track) XPath() string {
	return "Cisco-IOS-XR-um-track-cfg:tracks/track[track-name=" + t.TrackID + "]"
}

type Track struct {
	TrackID string    `json:"track-name"`
	Type    TrackType `json:"type"`
}

type TrackType struct {
	RTR uint32 `json:"rtr"`
}
