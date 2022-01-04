//
// (C) Copyright 2021 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package sysfs

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/lib/hardware"
	"github.com/daos-stack/daos/src/control/logging"
)

// NewProvider creates a new SysfsProvider.
func NewProvider(log logging.Logger) *Provider {
	return &Provider{
		root: "/sys",
		log:  log,
	}
}

// SysfsProvider provides system information from sysfs.
type Provider struct {
	log  logging.Logger
	root string
}

func (s *Provider) getRoot() string {
	if s.root == "" {
		s.root = "/sys"
	}
	return s.root
}

func (s *Provider) sysPath(pathElem ...string) string {
	pathElem = append([]string{s.getRoot()}, pathElem...)

	return filepath.Join(pathElem...)
}

// GetNetDevClass fetches the network device class for the given network interface.
func (s *Provider) GetNetDevClass(dev string) (hardware.NetDevClass, error) {
	if dev == "" {
		return 0, errors.New("device name required")
	}

	devClass, err := ioutil.ReadFile(s.sysPath("class", "net", dev, "type"))
	if err != nil {
		return 0, err
	}

	res, err := strconv.Atoi(strings.TrimSpace(string(devClass)))
	return hardware.NetDevClass(res), err
}

// GetTopology builds a minimal topology of network devices from the contents of sysfs.
func (s *Provider) GetTopology(ctx context.Context) (*hardware.Topology, error) {
	if s == nil {
		return nil, errors.New("sysfs provider is nil")
	}

	topo := &hardware.Topology{}

	err := filepath.Walk(s.sysPath("devices"), func(path string, fi os.FileInfo, err error) error {
		if fi == nil {
			return nil
		}

		if err != nil {
			return err
		}

		// Network devices will have the device/net subdirectory structure
		netDev, err := ioutil.ReadDir(filepath.Join(path, "device", "net"))
		if err != nil || len(netDev) == 0 {
			return nil
		}

		devName := filepath.Base(path)
		netDevName := netDev[0].Name()

		var devType hardware.DeviceType
		if netDevName == devName {
			devType = hardware.DeviceTypeNetInterface
		} else {
			devType = hardware.DeviceTypeOFIDomain
		}

		numaPath := filepath.Join(path, "device", "numa_node")
		numaStr, err := ioutil.ReadFile(numaPath)
		if err != nil {
			s.log.Debugf("couldn't read %q: %s", numaPath, err)
			return nil
		}

		numaID, err := strconv.Atoi(string(numaStr))
		if err != nil || numaID < 0 {
			s.log.Debugf("invalid NUMA node ID %q, using NUMA node 0", numaStr)
			numaID = 0
		}

		pciPath, err := filepath.EvalSymlinks(filepath.Join(path, "device"))
		if err != nil {
			s.log.Debugf("couldn't get PCI info: %s", err)
			return nil
		}

		pciAddr, err := common.NewPCIAddress(filepath.Base(pciPath))
		if err != nil {
			s.log.Debugf("%q not parsed as PCI address: %s", pciAddr, err)
			return nil
		}

		s.log.Debugf("adding device found at %q (type %s, NUMA node %d)", path)

		topo.AddDevice(uint(numaID), &hardware.PCIDevice{
			Name:    devName,
			Type:    devType,
			PCIAddr: *pciAddr,
		})

		return nil
	})

	if err == io.EOF || err == nil {
		return topo, nil
	}
	return nil, err
}
