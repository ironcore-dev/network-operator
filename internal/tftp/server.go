// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package tftpserver implements a simple TFTP server for serving boot scripts to devices during provisioning.
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
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

type Server struct {
	Client         client.Reader
	Logger         klog.Logger
	ValidateSource bool
	Port           int
}

func (s *Server) Start(ctx context.Context) error {
	readHandler := func(filename string, rf io.ReaderFrom) error {
		log := s.Logger.WithValues("filename", filename)

		xfer, ok := rf.(tftp.OutgoingTransfer)
		if !ok {
			log.Info("TFTP request rejected", "reason", "invalid transfer type")
			return errors.New("request denied")
		}

		srcIP := xfer.RemoteAddr().IP.String()
		if srcIP == "" {
			log.Info("TFTP request rejected", "reason", "missing client IP")
			return errors.New("request denied")
		}
		log = log.WithValues("clientIP", srcIP)

		serial := parseSerial(filename)
		if serial == "" {
			log.Info("TFTP request rejected", "reason", "missing serial in filename")
			return errors.New("request denied")
		}
		log = log.WithValues("serial", serial)

		var device *corev1.Device
		var err error
		if serial != "" {
			device, err = deviceutil.GetDeviceBySerial(ctx, s.Client, serial)
		} else {
			device, err = deviceutil.GetDeviceByEndpointIP(ctx, s.Client, srcIP)
		}
		if err != nil {
			log.Info("TFTP request rejected", "reason", "device not found")
			return errors.New("request denied")
		}

		if s.ValidateSource {
			deviceIP := device.EndpointIP()
			if deviceIP != srcIP {
				log.Info("TFTP request rejected", "reason", "client IP does not match device endpoint", "deviceIP", deviceIP)
				return errors.New("request denied")
			}

			deviceSerial := device.Status.SerialNumber
			if !strings.EqualFold(deviceSerial, serial) {
				log.Info("TFTP request rejected", "reason", "serial does not match device serial", "deviceSerial", device.Status.SerialNumber)
				return errors.New("request denied")
			}
		}

		bootScript := resolveBootScript(ctx, s.Client, device.Namespace, device.Spec.Provisioning)
		if len(bootScript) == 0 {
			log.Info("TFTP request rejected", "reason", "empty bootscript")
			return errors.New("request denied")
		}

		xfer.SetSize(int64(len(bootScript)))

		n, err := rf.ReadFrom(bytes.NewReader(bootScript))
		if err != nil {
			log.Error(err, "failed to send TFTP payload")
			return err
		}

		log.Info("TFTP payload delivered", "bytes", n)
		return nil
	}

	addr := fmt.Sprintf(":%d", s.Port)
	srv := tftp.NewServer(readHandler, nil)
	go func() {
		<-ctx.Done()
		srv.Shutdown()
	}()
	return srv.ListenAndServe(addr)
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
	fn = strings.Trim(strings.ToLower(strings.TrimSpace(fn)), "/")
	if n, ok := strings.CutPrefix(fn, "serial-"); ok {
		fn = n
	}
	if i := strings.IndexByte(fn, '.'); i >= 0 {
		fn = fn[:i]
	}
	return fn
}
