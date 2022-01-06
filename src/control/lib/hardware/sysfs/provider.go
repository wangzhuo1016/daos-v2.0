//
// (C) Copyright 2021 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package sysfs

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"

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

	err := filepath.Walk(s.sysPath("class"), func(path string, fi os.FileInfo, err error) error {
		if fi == nil {
			return nil
		}

		if err != nil {
			return err
		}

		allowedClasses := getFabricDevClasses()
		allowedClasses = append(allowedClasses, "net")
		allowed := false
		for _, class := range allowedClasses {
			if strings.HasPrefix(path, s.sysPath("class", class)) {
				allowed = true
				break
			}
		}

		if !allowed {
			return nil
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

		numaID, err := s.getNUMANode(path)
		if err != nil {
			s.log.Debug(err.Error())
			return nil
		}

		pciAddr, err := s.getPCIAddress(path)
		if err != nil {
			s.log.Debug(err.Error())
			return nil
		}

		s.log.Debugf("adding device found at %q (type %s, NUMA node %d)", path, devType, numaID)

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

func getFabricDevClasses() []string {
	return []string{
		"infiniband",
		"cxi",
	}
}

func (s *Provider) getNUMANode(path string) (uint, error) {
	numaPath := filepath.Join(path, "device", "numa_node")
	numaBytes, err := ioutil.ReadFile(numaPath)
	if err != nil {
		return 0, errors.Wrapf(err, "couldn't read %q", numaPath)
	}
	numaStr := strings.TrimSpace(string(numaBytes))

	numaID, err := strconv.Atoi(numaStr)
	if err != nil || numaID < 0 {
		s.log.Debugf("invalid NUMA node ID %q, using NUMA node 0", numaStr)
		numaID = 0
	}
	return uint(numaID), nil
}

func (s *Provider) getPCIAddress(path string) (*common.PCIAddress, error) {
	pciPath, err := filepath.EvalSymlinks(filepath.Join(path, "device"))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get PCI device")
	}

	pciAddr, err := common.NewPCIAddress(filepath.Base(pciPath))
	if err != nil {
		return nil, errors.Wrapf(err, "%q not parsed as PCI address", pciAddr)
	}

	return pciAddr, nil
}
