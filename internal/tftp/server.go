// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package tftpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	tftp "github.com/pin/tftp/v3"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
)

// Server represents the inline TFTP instance
type Server struct {
	addr   string
	verify bool
	reader client.Reader
	logger klog.Logger
}

// deviceEndpointIPField is the cache field index key used to look up Device objects by
// the IP part of spec.endpoint.address.
const deviceEndpointIPField = "device.endpoint.ip"

type manager interface {
	GetClient() client.Client
	GetFieldIndexer() client.FieldIndexer
}

func New(ctx context.Context, addr string, verify bool, mgr manager, logger klog.Logger) (*Server, error) {
	return &Server{addr: addr, verify: verify, reader: mgr.GetClient(), logger: logger}, nil
}

// Start runs the TFTP server until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	readHandler := func(filename string, rf io.ReaderFrom) error {
		srcIP := ""
		if xfer, ok := rf.(tftp.OutgoingTransfer); ok {
			ra := xfer.RemoteAddr()
			srcIP = ra.IP.String()
		}
		if srcIP == "" {
			s.logger.Info("rejecting TFTP request: missing client IP")
			return errors.New("missing client ip")
		}

		reqName := strings.Trim(strings.ToLower(strings.TrimSpace(filename)), "/")
		serial := parseSerial(reqName)

		var device *corev1.Device
		if s.verify {
			var err error
			if serial != "" {
				device, err = s.lookupBySerial(ctx, serial)
				if err != nil {
					return err
				}
				if device == nil {
					s.logger.Info("rejecting TFTP request: unknown serial", "serial", serial, "clientIP", srcIP)
					return errors.New("unknown serial")
				}
			} else {
				device, err = s.lookupByIP(ctx, srcIP)
				if err != nil {
					return err
				}
				if device == nil {
					s.logger.Info("rejecting TFTP request: no device found for client IP", "clientIP", srcIP)
					return errors.New("unknown ip")
				}
			}
			deviceIP := endpointIP(device.Spec.Endpoint.Address)
			if deviceIP != "" && deviceIP != srcIP {
				s.logger.Info("rejecting TFTP request: client IP does not match device endpoint", "serial", device.Status.SerialNumber, "deviceIP", deviceIP, "clientIP", srcIP)
				return errors.New("ip mismatch")
			}

			if serial != "" {
				deviceSerial := strings.ToLower(strings.TrimSpace(device.Status.SerialNumber))
				if serial != deviceSerial {
					s.logger.Info("rejecting TFTP request: parsed serial does not match device serial", "parsedSerial", serial, "deviceSerial", deviceSerial, "clientIP", srcIP)
					return errors.New("serial mismatch")
				}
			}
		} else {
			d, err := s.lookupByIP(ctx, srcIP)
			if err != nil {
				return err
			}
			device = d
		}

		if device == nil {
			return errors.New("no device found")
		}

		bootScript := resolveBootScript(ctx, s.reader, device.Namespace, device.Spec.Provisioning)
		if len(bootScript) == 0 {
			return errors.New("empty bootscript")
		}

		if xfer, ok := rf.(tftp.OutgoingTransfer); ok {
			xfer.SetSize(int64(len(bootScript)))
		}

		n, err := rf.ReadFrom(bytes.NewReader(bootScript))
		if err != nil {
			s.logger.Error(err, "failed to send TFTP payload", "clientIP", srcIP)
			return err
		}
		s.logger.Info("TFTP payload delivered", "bytes", n, "clientIP", srcIP, "serial", device.Status.SerialNumber, "requestedFilename", filename)
		return nil
	}

	writeHandler := func(filename string, wt io.WriterTo) error {
		return errors.New("write forbidden")
	}

	srv := tftp.NewServer(readHandler, writeHandler)

	go func() {
		<-ctx.Done()
	}()
	s.logger.Info("starting inline TFTP server", "address", s.addr, "verifyClient", s.verify)
	return srv.ListenAndServe(s.addr)
}

func (s *Server) lookupBySerial(ctx context.Context, serial string) (*corev1.Device, error) {
	list := &corev1.DeviceList{}
	if err := s.reader.List(ctx, list, client.MatchingLabels{corev1.DeviceSerialLabel: serial}); err != nil {
		s.logger.Error(err, "failed to look up device by serial", "serial", serial)
		return nil, fmt.Errorf("failed to look up device by serial %q: %w", serial, err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}

func (s *Server) lookupByIP(ctx context.Context, ip string) (*corev1.Device, error) {
	list := &corev1.DeviceList{}
	if err := s.reader.List(ctx, list, client.MatchingFields{deviceEndpointIPField: ip}); err != nil {
		s.logger.Error(err, "failed to look up device by client IP", "clientIP", ip)
		return nil, fmt.Errorf("failed to look up device by client IP %q: %w", ip, err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}

func endpointIP(address string) string {
	ip, _, _ := strings.Cut(address, ":")
	return ip
}

func resolveBootScript(ctx context.Context, r client.Reader, ns string, p *corev1.Provisioning) []byte {
	if p == nil {
		return nil
	}
	c := clientutil.NewClient(r, ns)
	data, err := c.Template(ctx, &p.BootScript)
	if err != nil {
		return nil
	}
	return data
}

func parseSerial(fn string) string {
	if n, ok := strings.CutPrefix(fn, "serial-"); ok {
		fn = n
	}
	if i := strings.IndexByte(fn, '.'); i >= 0 {
		fn = fn[:i]
	}
	return strings.TrimSpace(fn)
}
