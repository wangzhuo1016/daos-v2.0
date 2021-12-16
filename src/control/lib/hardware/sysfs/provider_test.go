//
// (C) Copyright 2021 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package sysfs

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/lib/hardware"
	"github.com/daos-stack/daos/src/control/logging"
)

func TestSysfs_NewProvider(t *testing.T) {
	log, buf := logging.NewTestLogger(t.Name())
	defer common.ShowBufferOnFailure(t, buf)

	p := NewProvider(log)

	if p == nil {
		t.Fatal("nil provider returned")
	}

	common.AssertEqual(t, "/sys", p.root, "")
}

func TestSysfs_Provider_GetNetDevClass(t *testing.T) {
	testDir, cleanupTestDir := common.CreateTestDir(t)
	defer cleanupTestDir()

	devs := map[string]uint32{
		"lo":   uint32(hardware.Loopback),
		"eth1": uint32(hardware.Ether),
	}

	for dev, class := range devs {
		path := filepath.Join(testDir, "class", "net", dev)
		os.MkdirAll(path, 0755)

		f, err := os.Create(filepath.Join(path, "type"))
		if err != nil {
			t.Fatal(err.Error())
		}

		_, err = f.WriteString(fmt.Sprintf("%d\n", class))
		f.Close()
		if err != nil {
			t.Fatal(err.Error())
		}
	}

	for name, tc := range map[string]struct {
		in        string
		expResult hardware.NetDevClass
		expErr    error
	}{
		"empty": {
			expErr: errors.New("device name required"),
		},
		"no such device": {
			in:     "fakedevice",
			expErr: errors.New("no such file"),
		},
		"loopback": {
			in:        "lo",
			expResult: hardware.NetDevClass(devs["lo"]),
		},
		"ether": {
			in:        "eth1",
			expResult: hardware.NetDevClass(devs["eth1"]),
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(name)
			defer common.ShowBufferOnFailure(t, buf)

			p := NewProvider(log)
			p.root = testDir

			result, err := p.GetNetDevClass(tc.in)

			common.CmpErr(t, tc.expErr, err)
			common.AssertEqual(t, tc.expResult, result, "")
		})
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := ioutil.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestProvider_GetTopology(t *testing.T) {
	for name, tc := range map[string]struct {
		setup     func(*testing.T, string)
		p         *Provider
		expResult *hardware.Topology
		expErr    error
	}{
		"nil": {
			expErr: errors.New("nil"),
		},
		"empty": {
			p:         &Provider{},
			expResult: &hardware.Topology{},
		},
		"no net devices": {
			setup: func(t *testing.T, root string) {
				for _, bus := range []string{"0000:00", "0000:01", "0000:02"} {
					path := filepath.Join(root, "devices", "pci"+bus, "0000:00:01.0", "0000:02:00.0")
					err := os.MkdirAll(path, 0755)
					if err != nil {
						t.Fatal(err)
					}
				}
			},
			p:         &Provider{},
			expResult: &hardware.Topology{},
		},
		"net device only": {
			setup: func(t *testing.T, root string) {
				pciPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "0000:02:00.0")
				path := filepath.Join(pciPath, "net", "net0")
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(pciPath, filepath.Join(path, "device")); err != nil {
					t.Fatal(err)
				}

				writeTestFile(t, filepath.Join(pciPath, "numa_node"), "2")
			},
			p: &Provider{},
			expResult: &hardware.Topology{
				NUMANodes: hardware.NodeMap{
					2: hardware.MockNUMANode(2, 0).
						WithDevices([]*hardware.PCIDevice{
							{
								Name:    "net0",
								Type:    hardware.DeviceTypeNetInterface,
								PCIAddr: *common.MustNewPCIAddress("0000:02:00.0"),
							},
						}),
				},
			},
		},
		"fabric device": {
			setup: func(t *testing.T, root string) {
				pciPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "0000:02:00.0")
				for _, path := range []string{
					filepath.Join(pciPath, "net", "net0"),
					filepath.Join(pciPath, "infiniband", "ib0"),
				} {
					if err := os.MkdirAll(path, 0755); err != nil {
						t.Fatal(err)
					}

					if err := os.Symlink(pciPath, filepath.Join(path, "device")); err != nil {
						t.Fatal(err)
					}
				}

				writeTestFile(t, filepath.Join(pciPath, "numa_node"), "2")
			},
			p: &Provider{},
			expResult: &hardware.Topology{
				NUMANodes: hardware.NodeMap{
					2: hardware.MockNUMANode(2, 0).
						WithDevices([]*hardware.PCIDevice{
							{
								Name:    "ib0",
								Type:    hardware.DeviceTypeOFIDomain,
								PCIAddr: *common.MustNewPCIAddress("0000:02:00.0"),
							},
							{
								Name:    "net0",
								Type:    hardware.DeviceTypeNetInterface,
								PCIAddr: *common.MustNewPCIAddress("0000:02:00.0"),
							},
						}),
				},
			},
		},
		"no NUMA node": {
			setup: func(t *testing.T, root string) {
				pciPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "0000:02:00.0")
				path := filepath.Join(pciPath, "net", "net0")
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(pciPath, filepath.Join(path, "device")); err != nil {
					t.Fatal(err)
				}

				writeTestFile(t, filepath.Join(pciPath, "numa_node"), "-1")
			},
			p: &Provider{},
			expResult: &hardware.Topology{
				NUMANodes: hardware.NodeMap{
					0: hardware.MockNUMANode(0, 0).
						WithDevices([]*hardware.PCIDevice{
							{
								Name:    "net0",
								Type:    hardware.DeviceTypeNetInterface,
								PCIAddr: *common.MustNewPCIAddress("0000:02:00.0"),
							},
						}),
				},
			},
		},
		"garbage NUMA file": {
			setup: func(t *testing.T, root string) {
				pciPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "0000:02:00.0")
				path := filepath.Join(pciPath, "net", "net0")
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(pciPath, filepath.Join(path, "device")); err != nil {
					t.Fatal(err)
				}

				writeTestFile(t, filepath.Join(pciPath, "numa_node"), "abcdef")
			},
			p: &Provider{},
			expResult: &hardware.Topology{
				NUMANodes: hardware.NodeMap{
					0: hardware.MockNUMANode(0, 0).
						WithDevices([]*hardware.PCIDevice{
							{
								Name:    "net0",
								Type:    hardware.DeviceTypeNetInterface,
								PCIAddr: *common.MustNewPCIAddress("0000:02:00.0"),
							},
						}),
				},
			},
		},
		"no NUMA file": {
			setup: func(t *testing.T, root string) {
				pciPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "0000:02:00.0")
				path := filepath.Join(pciPath, "net", "net0")
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(pciPath, filepath.Join(path, "device")); err != nil {
					t.Fatal(err)
				}
			},
			p:         &Provider{},
			expResult: &hardware.Topology{},
		},
		"no PCI device link": {
			setup: func(t *testing.T, root string) {
				pciPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "0000:02:00.0")
				path := filepath.Join(pciPath, "net", "net0")
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}

				writeTestFile(t, filepath.Join(pciPath, "numa_node"), "0")
			},
			p:         &Provider{},
			expResult: &hardware.Topology{},
		},
		"device link not valid PCI addr": {
			setup: func(t *testing.T, root string) {
				pciPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "junky")
				path := filepath.Join(pciPath, "net", "net0")
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(pciPath, filepath.Join(path, "device")); err != nil {
					t.Fatal(err)
				}

				writeTestFile(t, filepath.Join(pciPath, "numa_node"), "-1")
			},
			p:         &Provider{},
			expResult: &hardware.Topology{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(name)
			defer common.ShowBufferOnFailure(t, buf)

			testDir, cleanupTestDir := common.CreateTestDir(t)
			defer cleanupTestDir()

			if tc.setup == nil {
				tc.setup = func(t *testing.T, root string) {}
			}

			tc.setup(t, testDir)

			if tc.p != nil {
				tc.p.log = log

				// Mock out a fake sysfs in the testDir
				tc.p.root = testDir
			}

			result, err := tc.p.GetTopology(context.Background())

			common.CmpErr(t, tc.expErr, err)

			if diff := cmp.Diff(tc.expResult, result); diff != "" {
				t.Errorf("(-want, +got)\n%s\n", diff)
			}
		})
	}
}
