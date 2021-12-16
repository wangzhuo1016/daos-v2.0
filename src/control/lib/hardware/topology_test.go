//
// (C) Copyright 2021 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package hardware

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/daos-stack/daos/src/control/common"
)

func TestHardware_Topology_AllDevices(t *testing.T) {
	for name, tc := range map[string]struct {
		topo      *Topology
		expResult map[string]*PCIDevice
	}{
		"nil": {
			expResult: make(map[string]*PCIDevice),
		},
		"no NUMA nodes": {
			topo:      &Topology{},
			expResult: make(map[string]*PCIDevice),
		},
		"no PCI addrs": {
			topo: &Topology{
				NUMANodes: map[uint]*NUMANode{
					0: MockNUMANode(0, 8),
				},
			},
			expResult: make(map[string]*PCIDevice),
		},
		"single device": {
			topo: &Topology{
				NUMANodes: map[uint]*NUMANode{
					0: MockNUMANode(0, 8).WithDevices(
						[]*PCIDevice{
							mockPCIDevice("test").withType(DeviceTypeNetInterface),
						},
					),
				},
			},
			expResult: map[string]*PCIDevice{
				"test00": mockPCIDevice("test").withType(DeviceTypeNetInterface),
			},
		},
		"multi device": {
			topo: &Topology{
				NUMANodes: map[uint]*NUMANode{
					0: MockNUMANode(0, 8).WithDevices(
						[]*PCIDevice{
							mockPCIDevice("test0", 1, 1, 0).withType(DeviceTypeNetInterface),
							mockPCIDevice("test1", 1, 1, 0).withType(DeviceTypeOFIDomain),
							mockPCIDevice("test2", 1, 2, 1).withType(DeviceTypeNetInterface),
							mockPCIDevice("test3", 1, 2, 1).withType(DeviceTypeUnknown),
						},
					),
					1: MockNUMANode(1, 8).WithDevices(
						[]*PCIDevice{
							mockPCIDevice("test4", 2, 1, 1).withType(DeviceTypeNetInterface),
							mockPCIDevice("test5", 2, 1, 1).withType(DeviceTypeOFIDomain),
							mockPCIDevice("test6", 2, 2, 1).withType(DeviceTypeNetInterface),
						},
					),
				},
			},
			expResult: map[string]*PCIDevice{
				"test001": mockPCIDevice("test0", 1, 1, 0).withType(DeviceTypeNetInterface),
				"test101": mockPCIDevice("test1", 1, 1, 0).withType(DeviceTypeOFIDomain),
				"test201": mockPCIDevice("test2", 1, 2, 1).withType(DeviceTypeNetInterface),
				"test301": mockPCIDevice("test3", 1, 2, 1).withType(DeviceTypeUnknown),
				"test402": mockPCIDevice("test4", 2, 1, 1).withType(DeviceTypeNetInterface),
				"test502": mockPCIDevice("test5", 2, 1, 1).withType(DeviceTypeOFIDomain),
				"test602": mockPCIDevice("test6", 2, 2, 1).withType(DeviceTypeNetInterface),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := tc.topo.AllDevices()

			cmpOpts := []cmp.Option{
				cmpopts.IgnoreFields(PCIDevice{}, "NUMANode"),
			}
			if diff := cmp.Diff(tc.expResult, result, cmpOpts...); diff != "" {
				t.Fatalf("(-want, +got)\n%s\n", diff)
			}
		})
	}
}

func TestTopology_NumNUMANodes(t *testing.T) {
	for name, tc := range map[string]struct {
		topo      *Topology
		expResult int
	}{
		"nil": {},
		"empty": {
			topo: &Topology{},
		},
		"one": {
			topo: &Topology{
				NUMANodes: NodeMap{
					0: MockNUMANode(0, 8),
				},
			},
			expResult: 1,
		},
		"multiple": {
			topo: &Topology{
				NUMANodes: NodeMap{
					0: MockNUMANode(0, 8),
					1: MockNUMANode(1, 8),
					2: MockNUMANode(2, 8),
				},
			},
			expResult: 3,
		},
	} {
		t.Run(name, func(t *testing.T) {
			common.AssertEqual(t, tc.expResult, tc.topo.NumNUMANodes(), "")
		})
	}
}

func TestTopology_NumCoresPerNUMA(t *testing.T) {
	for name, tc := range map[string]struct {
		topo      *Topology
		expResult int
	}{
		"nil": {},
		"empty": {
			topo: &Topology{},
		},
		"no cores": {
			topo: &Topology{
				NUMANodes: NodeMap{
					0: MockNUMANode(0, 0),
				},
			},
		},
		"one NUMA": {
			topo: &Topology{
				NUMANodes: NodeMap{
					0: MockNUMANode(0, 6),
				},
			},
			expResult: 6,
		},
		"multiple NUMA": {
			topo: &Topology{
				NUMANodes: NodeMap{
					0: MockNUMANode(0, 8),
					1: MockNUMANode(1, 8),
					2: MockNUMANode(2, 8),
				},
			},
			expResult: 8,
		},
	} {
		t.Run(name, func(t *testing.T) {
			common.AssertEqual(t, tc.expResult, tc.topo.NumCoresPerNUMA(), "")
		})
	}
}

func TestHardware_Topology_AddDevice(t *testing.T) {
	for name, tc := range map[string]struct {
		topo      *Topology
		numaNode  uint
		device    *PCIDevice
		expResult *Topology
	}{
		"nil topology": {
			device: &PCIDevice{
				Name:    "test",
				PCIAddr: *MustNewPCIAddress("0000:00:00.1"),
			},
		},
		"nil input": {
			topo:      &Topology{},
			expResult: &Topology{},
		},
		"add to empty": {
			topo:     &Topology{},
			numaNode: 1,
			device: &PCIDevice{
				Name:    "test",
				PCIAddr: *MustNewPCIAddress("0000:00:00.1"),
			},
			expResult: &Topology{
				NUMANodes: NodeMap{
					1: MockNUMANode(1, 0).WithDevices([]*PCIDevice{
						{
							Name:    "test",
							PCIAddr: *MustNewPCIAddress("0000:00:00.1"),
						},
					}),
				},
			},
		},
		"add to existing node": {
			topo: &Topology{
				NUMANodes: NodeMap{
					1: MockNUMANode(1, 6).WithDevices([]*PCIDevice{
						{
							Name:    "test0",
							PCIAddr: *MustNewPCIAddress("0000:00:00.1"),
						},
					}),
				},
			},
			numaNode: 1,
			device: &PCIDevice{
				Name:    "test1",
				PCIAddr: *MustNewPCIAddress("0000:00:00.2"),
			},
			expResult: &Topology{
				NUMANodes: NodeMap{
					1: MockNUMANode(1, 6).WithDevices([]*PCIDevice{
						{
							Name:    "test0",
							PCIAddr: *MustNewPCIAddress("0000:00:00.1"),
						},
						{
							Name:    "test1",
							PCIAddr: *MustNewPCIAddress("0000:00:00.2"),
						},
					}),
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.topo.AddDevice(tc.numaNode, tc.device)

			if diff := cmp.Diff(tc.expResult, tc.topo); diff != "" {
				t.Fatalf("(-want, +got)\n%s\n", diff)
			}
		})
	}
}

func TestHardware_Topology_Merge(t *testing.T) {
	testNuma := []*NUMANode{
		MockNUMANode(1, 4).
			WithDevices([]*PCIDevice{
				{
					Name:      "test0",
					PCIAddr:   *MustNewPCIAddress("0000:00:00.1"),
					LinkSpeed: 60,
				},
			}).
			WithCPUCores([]CPUCore{}).
			WithPCIBuses([]*PCIBus{
				{
					LowAddress:  *MustNewPCIAddress("0000:00:00.0"),
					HighAddress: *MustNewPCIAddress("0000:05:00.0"),
				},
			}),
		MockNUMANode(2, 4).
			WithDevices([]*PCIDevice{
				{
					Name:    "test1",
					PCIAddr: *MustNewPCIAddress("0000:0a:00.1"),
				},
			}).
			WithCPUCores([]CPUCore{}).
			WithPCIBuses([]*PCIBus{
				{
					LowAddress:  *MustNewPCIAddress("0000:05:00.0"),
					HighAddress: *MustNewPCIAddress("0000:0f:00.0"),
				},
			}),
	}

	for name, tc := range map[string]struct {
		topo      *Topology
		input     *Topology
		expResult *Topology
	}{
		"nil base": {
			input: &Topology{},
		},
		"nil input": {
			topo:      &Topology{},
			expResult: &Topology{},
		},
		"all empties": {
			topo:      &Topology{},
			input:     &Topology{},
			expResult: &Topology{},
		},
		"add to empty": {
			topo: &Topology{},
			input: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: testNuma[0],
				},
			},
			expResult: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: testNuma[0],
				},
			},
		},
		"no intersection": {
			topo: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: testNuma[0],
				},
			},
			input: &Topology{
				NUMANodes: NodeMap{
					testNuma[1].ID: testNuma[1],
				},
			},
			expResult: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: testNuma[0],
					testNuma[1].ID: testNuma[1],
				},
			},
		},
		"add to same NUMA node": {
			topo: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: testNuma[0],
				},
			},
			input: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: MockNUMANode(testNuma[0].ID, 0).
						WithDevices([]*PCIDevice{
							{
								Name:    "test1",
								Type:    DeviceTypeNetInterface,
								PCIAddr: *MustNewPCIAddress("0000:00:00.2"),
							},
						}).
						WithCPUCores([]CPUCore{
							{
								ID: 4,
							},
						}).
						WithPCIBuses([]*PCIBus{
							{
								LowAddress:  *MustNewPCIAddress("0000:0f:00.0"),
								HighAddress: *MustNewPCIAddress("0000:20:00.0"),
							},
						}),
				},
			},
			expResult: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: MockNUMANode(testNuma[0].ID, 5).
						WithDevices([]*PCIDevice{
							testNuma[0].PCIDevices[*MustNewPCIAddress("0000:00:00.1")][0],
							{
								Name:    "test1",
								Type:    DeviceTypeNetInterface,
								PCIAddr: *MustNewPCIAddress("0000:00:00.2"),
							},
						}).
						WithPCIBuses([]*PCIBus{
							testNuma[0].PCIBuses[0],
							{
								LowAddress:  *MustNewPCIAddress("0000:0f:00.0"),
								HighAddress: *MustNewPCIAddress("0000:20:00.0"),
							},
						}),
				},
			},
		},
		"update": {
			topo: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: testNuma[0],
				},
			},
			input: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: MockNUMANode(testNuma[0].ID, 5).
						WithDevices([]*PCIDevice{
							{
								Name:    "test0",
								Type:    DeviceTypeNetInterface,
								PCIAddr: *MustNewPCIAddress("0000:00:00.1"),
							},
							{
								Name:      "test1",
								Type:      DeviceTypeNetInterface,
								PCIAddr:   *MustNewPCIAddress("0000:00:00.2"),
								LinkSpeed: 75,
							},
						}).
						WithPCIBuses([]*PCIBus{
							testNuma[0].PCIBuses[0],
							{
								LowAddress:  *MustNewPCIAddress("0000:0f:00.0"),
								HighAddress: *MustNewPCIAddress("0000:20:00.0"),
							},
						}),
				},
			},
			expResult: &Topology{
				NUMANodes: NodeMap{
					testNuma[0].ID: MockNUMANode(testNuma[0].ID, 5).
						WithDevices([]*PCIDevice{
							testNuma[0].PCIDevices[*MustNewPCIAddress("0000:00:00.1")][0],
							{
								Name:      "test1",
								Type:      DeviceTypeNetInterface,
								PCIAddr:   *MustNewPCIAddress("0000:00:00.2"),
								LinkSpeed: 75,
							},
						}).
						WithPCIBuses([]*PCIBus{
							testNuma[0].PCIBuses[0],
							{
								LowAddress:  *MustNewPCIAddress("0000:0f:00.0"),
								HighAddress: *MustNewPCIAddress("0000:20:00.0"),
							},
						}),
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.topo.Merge(tc.input)

			if diff := cmp.Diff(tc.expResult, tc.topo, common.CmpOptIgnoreField("NUMANode")); diff != "" {
				t.Fatalf("(-want, +got)\n%s\n", diff)
			}
		})
	}
}

func TestHardware_DeviceType_String(t *testing.T) {
	for name, tc := range map[string]struct {
		devType   DeviceType
		expResult string
	}{
		"network": {
			devType:   DeviceTypeNetInterface,
			expResult: "network interface",
		},
		"OFI domain": {
			devType:   DeviceTypeOFIDomain,
			expResult: "OFI domain",
		},
		"unknown": {
			devType:   DeviceTypeUnknown,
			expResult: "unknown device type",
		},
		"not recognized": {
			devType:   DeviceType(0xffffffff),
			expResult: "unknown device type",
		},
	} {
		t.Run(name, func(t *testing.T) {
			common.AssertEqual(t, tc.expResult, tc.devType.String(), "")
		})
	}
}
