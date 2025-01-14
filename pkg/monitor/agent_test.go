/*
Copyright 2021 The Everoute Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package monitor

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	ovsdb "github.com/contiv/libovsdb"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/everoute/everoute/pkg/agent/datapath"
	agentv1alpha1 "github.com/everoute/everoute/pkg/apis/agent/v1alpha1"
	"github.com/everoute/everoute/pkg/client/clientset_generated/clientset/scheme"
	"github.com/everoute/everoute/pkg/types"
)

const (
	timeout  = time.Second * 60
	interval = time.Millisecond * 250
)

type Iface struct {
	IfaceName  string
	IfaceType  string
	MacAddr    net.HardwareAddr
	OfPort     uint32
	externalID map[string]string
}

var (
	k8sClient                  client.Client
	ovsClient                  *ovsdb.OvsdbClient
	agentName                  string
	monitor                    *AgentMonitor
	stopChan                   chan struct{}
	ofPortIPAddressMonitorChan chan map[string]net.IP
	localEndpointLock          sync.RWMutex
	localEndpointMap           map[uint32]net.HardwareAddr
)

func TestMain(m *testing.M) {
	k8sClient = fake.NewFakeClientWithScheme(scheme.Scheme)

	var err error

	ovsClient, err = ovsdb.ConnectUnix(ovsdb.DEFAULT_SOCK)
	if err != nil {
		klog.Fatalf("fail to connect ovs client: %s", err)
	}

	monitor, stopChan, ofPortIPAddressMonitorChan = startAgentMonitor(k8sClient)
	agentName = monitor.Name()

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestAgentMonitor(t *testing.T) {
	RegisterTestingT(t)

	brName := string(uuid.NewUUID())
	fakeportName := string(uuid.NewUUID())
	portName := "if-" + strings.Split(string(uuid.NewUUID()), "-")[0]
	portPeerName := "if-" + strings.Split(string(uuid.NewUUID()), "-")[0]
	ifaceName := portName
	externalIDs := map[string]string{"everoute.agent.monitor.externalID.name": "everoute.agent.monitor.externalID.value"}

	t.Logf("create new bridge %s", brName)
	Expect(createBridge(ovsClient, brName)).Should(Succeed())

	t.Run("monitor should create new bridge", func(t *testing.T) {
		Eventually(func() error {
			_, err := getBridge(k8sClient, brName)
			return err
		}, timeout, interval).Should(Succeed())
	})

	t.Logf("create new port %s", portName)
	Expect(createVethPair(portName, portPeerName)).Should(Succeed())
	Expect(createPort(ovsClient, brName, portName)).Should(Succeed())

	t.Run("monitor should create new port", func(t *testing.T) {
		Eventually(func() error {
			_, err := getPort(k8sClient, brName, portName)
			return err
		}, timeout, interval).Should(Succeed())
	})

	t.Logf("create new fake port %s", fakeportName)
	Expect(createPort(ovsClient, brName, fakeportName)).Should(Succeed())

	t.Run("monitor should create new port", func(t *testing.T) {
		Eventually(func() error {
			_, err := getPort(k8sClient, brName, fakeportName)
			return err
		}, timeout, interval).Should(Succeed())
	})

	t.Run("interface with error should not appear in agentInfo", func(t *testing.T) {
		Eventually(func() error {
			monitor.ipCacheLock.RLock()
			defer monitor.ipCacheLock.RUnlock()
			agentInfo, _ := monitor.getAgentInfo()
			for _, br := range agentInfo.OVSInfo.Bridges {
				for _, port := range br.Ports {
					for _, iface := range port.Interfaces {
						if iface.Name == fakeportName {
							return fmt.Errorf("error")
						}
					}
				}
			}
			return nil
		}, timeout, interval).Should(Succeed())
	})

	t.Logf("update port %s externalIDs to %+v", portName, externalIDs)
	Expect(updatePort(ovsClient, portName, externalIDs)).Should(Succeed())

	t.Run("monitor should update port externalID", func(t *testing.T) {
		Eventually(func() map[string]string {
			port, _ := getPort(k8sClient, brName, portName)
			return port.ExternalIDs
		}, timeout, interval).Should(Equal(externalIDs))
	})

	t.Logf("update interface %s externalIDs to %+v", ifaceName, externalIDs)
	Expect(updateInterface(ovsClient, ifaceName, externalIDs)).Should(Succeed())

	t.Run("monitor should update interface externalID", func(t *testing.T) {
		Eventually(func() map[string]string {
			iface, _ := getIface(k8sClient, brName, portName, ifaceName)
			return iface.ExternalIDs
		}, timeout, interval).Should(Equal(externalIDs))
	})

	t.Logf("delete port %s on bridge %s", portName, brName)
	Expect(deletePort(ovsClient, brName, portName)).Should(Succeed())

	t.Run("monitor should delete port", func(t *testing.T) {
		Eventually(func() bool {
			_, err := getPort(k8sClient, brName, portName)
			return isNotFoundError(err)
		}, timeout, interval).Should(BeTrue())
	})

	t.Logf("delete bridge %s", brName)
	Expect(deleteBridge(ovsClient, brName)).Should(Succeed())

	t.Run("monitor should delete bridge", func(t *testing.T) {
		Eventually(func() bool {
			_, err := getBridge(k8sClient, brName)
			return isNotFoundError(err)
		}, timeout, interval).Should(BeTrue())
	})
}

func TestAgentMonitorIpAddressLearning(t *testing.T) {
	RegisterTestingT(t)
	brName := string(uuid.NewUUID())

	t.Logf("create new bridge %s", brName)
	Expect(createBridge(ovsClient, brName)).Should(Succeed())

	var ofPort1 uint32 = 1
	var ipAddr1 = net.ParseIP("10.10.10.1")
	var ipAddr2 = net.ParseIP("10.10.10.2")

	t.Logf("Add OfPort %d, IpAddress %v.", ofPort1, ipAddr1)
	Expect(addOfPortIPAddress(brName, ofPort1, ipAddr1, ofPortIPAddressMonitorChan)).Should(Succeed())

	t.Run("Monitor should learning ofPort to IpAddress mapping.", func(t *testing.T) {
		Eventually(func() bool {
			monitor.ipCacheLock.RLock()
			defer monitor.ipCacheLock.RUnlock()
			ipSet := toIPSet(monitor.ipCache[fmt.Sprintf("%s-%d", brName, ofPort1)])
			return ipSet.Has(ipAddr1.String())
		}, timeout, interval).Should(Equal(false))
	})

	t.Logf("Add another ovsPort related IpAddress %v.", ipAddr2)
	Expect(updateIPAddress(brName, ofPort1, ipAddr2, ofPortIPAddressMonitorChan)).Should(Succeed())

	t.Run("Monitor should update learned OfPort to IpAddress mapping.", func(t *testing.T) {
		Eventually(func() bool {
			monitor.ipCacheLock.RLock()
			defer monitor.ipCacheLock.RUnlock()
			ipSet := toIPSet(monitor.ipCache[fmt.Sprintf("%s-%d", brName, ofPort1)])
			return ipSet.Has(ipAddr2.String())
		}, timeout, interval).Should(Equal(false))
	})
}

func TestOvsDbEventHandler(t *testing.T) {
	RegisterTestingT(t)

	bridgeName := string(uuid.NewUUID())
	ep1Port := "ep1"
	ep1MacAddrStr := "00:11:11:11:11:11"
	ep1InterfaceExternalIds := map[string]string{"attached-mac": ep1MacAddrStr}
	ep1Iface := Iface{
		IfaceName:  "ep1",
		IfaceType:  "internal",
		OfPort:     uint32(11),
		externalID: ep1InterfaceExternalIds,
	}

	t.Logf("create new bridge %s", bridgeName)
	Expect(createBridge(ovsClient, bridgeName)).Should(Succeed())

	// Add local endpoint, set attached interface externalIDs
	Expect(createOvsPort(bridgeName, ep1Port, []Iface{ep1Iface}, 0)).Should(Succeed())

	t.Run("Add local endpoint ep1", func(t *testing.T) {
		Eventually(func() string {
			localEndpointLock.Lock()
			defer localEndpointLock.Unlock()

			if ep1MacAddr, ok := localEndpointMap[ep1Iface.OfPort]; ok {
				return ep1MacAddr.String()
			}

			return ""
		}, timeout, interval).Should(Equal(ep1MacAddrStr))
	})

	// Delete local endpoint
	Expect(deletePort(ovsClient, bridgeName, ep1Port, ep1Iface.IfaceName)).Should(Succeed())

	t.Run("Delete local endpoint ep1", func(t *testing.T) {
		Eventually(func() bool {
			localEndpointLock.Lock()
			defer localEndpointLock.Unlock()

			if ep1MacAddr, ok := localEndpointMap[ep1Iface.OfPort]; ok {
				if ep1MacAddr.String() == ep1MacAddrStr {
					return false
				}
			}
			return true
		}, timeout, interval).Should(Equal(true))
	})

	// Add vethpair type interface, convert to endpoint, update interface MacAddr
	vethPortName := "if-" + strings.Split(string(uuid.NewUUID()), "-")[0]
	vethPortPeerName := "if-" + strings.Split(string(uuid.NewUUID()), "-")[0]
	vethIfaceName := vethPortName

	t.Logf("create vethpair port %s", vethPortName)
	Expect(createVethPair(vethPortName, vethPortPeerName)).Should(Succeed())
	Expect(createPort(ovsClient, bridgeName, vethPortName)).Should(Succeed())

	t.Run("monitor should create new veth port", func(t *testing.T) {
		Eventually(func() error {
			_, err := getPort(k8sClient, bridgeName, vethPortName)
			return err
		}, timeout, interval).Should(Succeed())
	})

	t.Run("monitor update veth interface test", func(t *testing.T) {
		Eventually(func() bool {
			localEndpointLock.Lock()
			defer localEndpointLock.Unlock()
			iface, _ := getIface(k8sClient, bridgeName, vethPortName, vethIfaceName)
			if vethIfaceMacAddr, ok := localEndpointMap[uint32(iface.Ofport)]; ok {
				if vethIfaceMacAddr.String() == iface.Mac {
					return true
				}
			}
			return false
		}, timeout, interval).Should(Equal(true))
	})

	t.Logf("delete port %s on bridge %s", vethPortName, bridgeName)
	Expect(deletePort(ovsClient, bridgeName, vethPortName)).Should(Succeed())

	t.Run("monitor delete veth port", func(t *testing.T) {
		Eventually(func() bool {
			_, err := getPort(k8sClient, bridgeName, vethPortName)
			return isNotFoundError(err)
		}, timeout, interval).Should(BeTrue())
	})

	internalPortName := "cl"
	internalIfaceName := internalPortName

	t.Logf("create internal port %s", internalPortName)
	internalIface := Iface{
		IfaceName: internalIfaceName,
		IfaceType: "internal",
		OfPort:    uint32(22),
	}
	Expect(createOvsPort(bridgeName, internalPortName, []Iface{internalIface}, 0)).Should(Succeed())

	t.Run("Add internal endpoint", func(t *testing.T) {
		Eventually(func() bool {
			localEndpointLock.Lock()
			defer localEndpointLock.Unlock()
			iface, _ := getIface(k8sClient, bridgeName, internalPortName, internalPortName)
			if internalIfaceMacAddr, ok := localEndpointMap[internalIface.OfPort]; ok {
				if internalIfaceMacAddr.String() == iface.Mac {
					return true
				}
			}
			return false
		}, timeout, interval).Should(Equal(true))
	})

	t.Logf("delete port %s on bridge %s", internalPortName, bridgeName)
	Expect(deletePort(ovsClient, bridgeName, internalPortName)).Should(Succeed())

	t.Run("Delete internal endpoint", func(t *testing.T) {
		Eventually(func() bool {
			_, err := getPort(k8sClient, bridgeName, internalPortName)
			return isNotFoundError(err)
		}, timeout, interval).Should(BeTrue())
	})
}

func getOvsDBInterfaceInfo(opStr string, interfaces []Iface) ([]ovsdb.UUID, []ovsdb.Operation) {
	var intfOperations []ovsdb.Operation
	intfUUID := []ovsdb.UUID{}

	for _, iface := range interfaces {
		intfUUIDStr := fmt.Sprintf("Intf%s", iface.IfaceName)
		intfUUID = append(intfUUID, ovsdb.UUID{GoUuid: intfUUIDStr})

		intf := make(map[string]interface{})
		intf["name"] = iface.IfaceName
		intf["type"] = iface.IfaceType
		intf["ofport"] = float64(iface.OfPort)
		if iface.externalID != nil {
			ovsExternalIDs, _ := ovsdb.NewOvsMap(iface.externalID)
			intf["external_ids"] = ovsExternalIDs
		}

		intfOp := ovsdb.Operation{
			Op:       opStr,
			Table:    "Interface",
			Row:      intf,
			UUIDName: intfUUIDStr,
		}

		intfOperations = append(intfOperations, intfOp)
	}

	return intfUUID, intfOperations
}

func createVethPair(vethName, peerName string) error {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: vethName, TxQLen: 0},
		PeerName:  peerName}
	if err := netlink.LinkAdd(veth); err != nil {
		return err
	}
	return nil
}

func createOvsPort(bridgeName, portName string, interfaces []Iface, vlanTag uint) error {
	var err error
	portUUIDStr := portName
	portUUID := []ovsdb.UUID{{GoUuid: portUUIDStr}}
	opStr := "insert"

	// Add interface to interfaces table
	intfUUID, intfOperations := getOvsDBInterfaceInfo(opStr, interfaces)

	// Insert a row in Port table
	port := make(map[string]interface{})
	port["name"] = portName
	if vlanTag != 0 {
		port["vlan_mode"] = "access"
		port["tag"] = vlanTag
	} else {
		port["vlan_mode"] = "trunk"
	}

	port["interfaces"], err = ovsdb.NewOvsSet(intfUUID)
	if err != nil {
		return err
	}

	// Add an entry in Port table
	portOp := ovsdb.Operation{
		Op:       opStr,
		Table:    "Port",
		Row:      port,
		UUIDName: portUUIDStr,
	}

	// mutate the Ports column of the row in the Bridge table
	mutateSet, _ := ovsdb.NewOvsSet(portUUID)
	mutation := ovsdb.NewMutation("ports", opStr, mutateSet)
	condition := ovsdb.NewCondition("name", "==", bridgeName)
	mutateOp := ovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	var operations []ovsdb.Operation
	operations = append(operations, intfOperations...)
	operations = append(operations, portOp, mutateOp)

	// Perform OVS transaction
	_, err = ovsdbTransact(ovsClient, "Open_vSwitch", operations...)

	return err
}

func updateInterface(client *ovsdb.OvsdbClient, ifaceName string, externalIDs map[string]string) error {
	if externalIDs == nil {
		externalIDs = make(map[string]string)
	}
	ovsExternalIDs, _ := ovsdb.NewOvsMap(externalIDs)

	portOperation := ovsdb.Operation{
		Op:    "update",
		Table: "Interface",
		Row: map[string]interface{}{
			"external_ids": ovsExternalIDs,
		},
		Where: []interface{}{[]interface{}{"name", "==", ifaceName}},
	}

	_, err := ovsdbTransact(client, "Open_vSwitch", portOperation)
	return err
}

func addOfPortIPAddress(brName string, ofPort uint32, ipAddr net.IP, ofPortIPAddressMonitorChan chan map[string]net.IP) error {
	ofPortInfo := map[string]net.IP{fmt.Sprintf("%s-%d", brName, ofPort): ipAddr}
	ofPortIPAddressMonitorChan <- ofPortInfo
	return nil
}

func updateIPAddress(brName string, ofPort uint32, newIPAddr net.IP, ofPortIPAddressMonitorChan chan map[string]net.IP) error {
	monitor.ipCacheLock.RLock()
	defer monitor.ipCacheLock.RUnlock()

	ofPortInfo := map[string]net.IP{
		fmt.Sprintf("%s-%d", brName, ofPort): newIPAddr,
	}
	ofPortIPAddressMonitorChan <- ofPortInfo
	return nil
}

const emptyUUID = "00000000-0000-0000-0000-000000000000"

func createBridge(client *ovsdb.OvsdbClient, brName string) error {
	bridgeOperation := ovsdb.Operation{
		Op:       "insert",
		Table:    "Bridge",
		UUIDName: "dummy",
		Row: map[string]interface{}{
			"name": brName,
		},
	}

	mutateOperation := ovsdb.Operation{
		Op:        "mutate",
		Table:     "Open_vSwitch",
		Mutations: []interface{}{[]interface{}{"bridges", "insert", ovsdb.UUID{GoUuid: "dummy"}}},
		Where:     []interface{}{[]interface{}{"_uuid", "excludes", ovsdb.UUID{GoUuid: emptyUUID}}},
	}

	_, err := ovsdbTransact(client, "Open_vSwitch", bridgeOperation, mutateOperation)
	return err
}

func deleteBridge(client *ovsdb.OvsdbClient, brName string) error {
	brUUID, err := getMemberUUID(client, "Bridge", brName)
	if err != nil {
		return fmt.Errorf("can't found uuid of bridge %s: %s", brName, err)
	}

	bridgeOperation := ovsdb.Operation{
		Op:    "delete",
		Table: "Bridge",
		Where: []interface{}{[]interface{}{"name", "==", brName}},
	}

	mutateOperation := ovsdb.Operation{
		Op:        "mutate",
		Table:     "Open_vSwitch",
		Mutations: []interface{}{[]interface{}{"bridges", "delete", brUUID}},
		Where:     []interface{}{[]interface{}{"_uuid", "excludes", ovsdb.UUID{GoUuid: emptyUUID}}},
	}

	_, err = ovsdbTransact(client, "Open_vSwitch", bridgeOperation, mutateOperation)
	return err
}

// createPort also create an interface with the same name
func createPort(client *ovsdb.OvsdbClient, brName, portName string) error {
	ifaceOperation := ovsdb.Operation{
		Op:    "insert",
		Table: "Interface",
		Row: map[string]interface{}{
			"name": portName,
		},
		UUIDName: "ifacedummy",
	}

	portOperation := ovsdb.Operation{
		Op:       "insert",
		Table:    "Port",
		UUIDName: "dummy",
		Row: map[string]interface{}{
			"name":       portName,
			"interfaces": ovsdb.UUID{GoUuid: "ifacedummy"},
		},
	}

	mutateOperation := ovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{[]interface{}{"ports", "insert", ovsdb.UUID{GoUuid: "dummy"}}},
		Where:     []interface{}{[]interface{}{"name", "==", brName}},
	}

	_, err := ovsdbTransact(client, "Open_vSwitch", ifaceOperation, portOperation, mutateOperation)
	return err
}

func updatePort(client *ovsdb.OvsdbClient, portName string, externalIDs map[string]string) error {
	if externalIDs == nil {
		externalIDs = make(map[string]string)
	}
	ovsExternalIDs, _ := ovsdb.NewOvsMap(externalIDs)

	portOperation := ovsdb.Operation{
		Op:    "update",
		Table: "Port",
		Row: map[string]interface{}{
			"external_ids": ovsExternalIDs,
		},
		Where: []interface{}{[]interface{}{"name", "==", portName}},
	}

	_, err := ovsdbTransact(client, "Open_vSwitch", portOperation)
	return err
}

func deletePort(client *ovsdb.OvsdbClient, brName, portName string, ifaceNames ...string) error {
	portUUID, err := getMemberUUID(client, "Port", portName)
	if err != nil {
		return fmt.Errorf("can't found uuid of port %s: %s", portName, err)
	}

	if len(ifaceNames) == 0 {
		// delete port default iface if ifaceNames not specific
		ifaceNames = []string{portName}
	}
	operations := make([]ovsdb.Operation, 0, len(ifaceNames)+2)

	for _, ifaceName := range ifaceNames {
		ifaceOperation := ovsdb.Operation{
			Op:    "delete",
			Table: "Interface",
			Where: []interface{}{[]interface{}{"name", "==", ifaceName}},
		}
		operations = append(operations, ifaceOperation)
	}

	portOperation := ovsdb.Operation{
		Op:    "delete",
		Table: "Port",
		Where: []interface{}{[]interface{}{"name", "==", portName}},
	}
	operations = append(operations, portOperation)

	mutateOperation := ovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{[]interface{}{"ports", "delete", portUUID}},
		Where:     []interface{}{[]interface{}{"name", "==", brName}},
	}
	operations = append(operations, mutateOperation)

	_, err = ovsdbTransact(client, "Open_vSwitch", operations...)
	return err
}

func getMemberUUID(client *ovsdb.OvsdbClient, tableName, memberName string) (ovsdb.UUID, error) {
	selectOperation := ovsdb.Operation{
		Op:    "select",
		Table: tableName,
		Where: []interface{}{[]interface{}{"name", "==", memberName}},
	}

	result, err := ovsdbTransact(client, "Open_vSwitch", selectOperation)
	if err != nil {
		return ovsdb.UUID{}, err
	}

	if len(result[0].Rows) == 0 {
		return ovsdb.UUID{}, fmt.Errorf("no member name with %s found in table %s", memberName, tableName)
	}

	return ovsdb.UUID{
		GoUuid: result[0].Rows[0]["_uuid"].([]interface{})[1].(string),
	}, nil
}

func ovsdbTransact(client *ovsdb.OvsdbClient, database string, operation ...ovsdb.Operation) ([]ovsdb.OperationResult, error) {
	results, err := client.Transact(database, operation...)
	for item, result := range results {
		if result.Error != "" {
			return results, fmt.Errorf("operator %v: %s, details: %s", operation[item], result.Error, result.Details)
		}
	}

	return results, err
}

func getBridge(client client.Client, brName string) (*agentv1alpha1.OVSBridge, error) {
	agentInfo := &agentv1alpha1.AgentInfo{}
	err := client.Get(context.Background(), k8stypes.NamespacedName{Name: agentName}, agentInfo)
	if err != nil {
		return nil, err
	}

	for _, bridge := range agentInfo.OVSInfo.Bridges {
		if bridge.Name == brName {
			return &bridge, nil
		}
	}

	return nil, notFoundError(fmt.Errorf("bridge %s not found in agentInfo", brName))
}

func getPort(client client.Client, brName, portName string) (*agentv1alpha1.OVSPort, error) {
	bridge, err := getBridge(client, brName)
	if err != nil {
		return nil, err
	}

	for _, port := range bridge.Ports {
		if port.Name == portName {
			return &port, nil
		}
	}

	return nil, notFoundError(fmt.Errorf("port %s not found in agentInfo", portName))
}

func getIface(client client.Client, brName, portName, ifaceName string) (*agentv1alpha1.OVSInterface, error) {
	port, err := getPort(client, brName, portName)
	if err != nil {
		return nil, err
	}

	for _, iface := range port.Interfaces {
		if iface.Name == ifaceName {
			return &iface, nil
		}
	}

	return nil, notFoundError(fmt.Errorf("port %s not found in agentInfo", ifaceName))
}

type notFoundError error

func isNotFoundError(err error) bool {
	switch err.(type) {
	case notFoundError:
		return true
	default:
		return false
	}
}

func startAgentMonitor(k8sClient client.Client) (*AgentMonitor, chan struct{}, chan map[string]net.IP) {
	ofPortIPAddressMonitorChan = make(chan map[string]net.IP, 1024)
	localEndpointMap = make(map[uint32]net.HardwareAddr)

	monitor, err := NewAgentMonitor(k8sClient, ofPortIPAddressMonitorChan)
	if err != nil {
		klog.Fatalf("fail to create agentMonitor: %s", err)
	}

	monitor.RegisterOvsdbEventHandler(OvsdbEventHandlerFuncs{
		LocalEndpointAddFunc: func(endpoint datapath.Endpoint) {
			localEndpointLock.Lock()
			defer localEndpointLock.Unlock()

			localEndpointMap[endpoint.PortNo], _ = net.ParseMAC(endpoint.MacAddrStr)
		},
		LocalEndpointDeleteFunc: func(endpoint datapath.Endpoint) {
			localEndpointLock.Lock()
			defer localEndpointLock.Unlock()

			delete(localEndpointMap, endpoint.PortNo)
		},
		LocalEndpointUpdateFunc: func(newEndpoint, oldEndpoint datapath.Endpoint) {
			localEndpointLock.Lock()
			defer localEndpointLock.Unlock()

			delete(localEndpointMap, oldEndpoint.PortNo)
			localEndpointMap[newEndpoint.PortNo], _ = net.ParseMAC(newEndpoint.MacAddrStr)
		},
	})

	stopChan := make(chan struct{})
	go monitor.Run(stopChan)

	return monitor, stopChan, ofPortIPAddressMonitorChan
}

// create or update agntinfo with giving ofport and IPAddr
func setOfportIPAddr(k8sClient client.Client, brName string, ofport int32, ipAddr []types.IPAddress) error {
	var ctx = context.Background()
	var agentInfoOld = &agentv1alpha1.AgentInfo{}

	var agentInfo = &agentv1alpha1.AgentInfo{
		OVSInfo: agentv1alpha1.OVSInfo{
			Bridges: []agentv1alpha1.OVSBridge{
				{
					Name: brName,
					Ports: []agentv1alpha1.OVSPort{
						{
							Interfaces: []agentv1alpha1.OVSInterface{
								{
									Ofport: ofport,
									IPMap: map[types.IPAddress]metav1.Time{
										ipAddr[0]: {},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	agentInfo.Name = agentName

	err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: agentName}, agentInfoOld)
	if errors.IsNotFound(err) {
		if err = k8sClient.Create(ctx, agentInfo); err != nil {
			return fmt.Errorf("couldn't create agent %s agentinfo: %s", agentName, err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("couldn't fetch agent %s agentinfo: %s", agentName, err)
	}

	agentInfo.ObjectMeta = agentInfoOld.ObjectMeta
	err = k8sClient.Update(ctx, agentInfo)
	if err != nil {
		return fmt.Errorf("couldn't update agent %s agentinfo: %s", agentName, err)
	}

	return nil
}

func toIPSet(ipmap map[types.IPAddress]metav1.Time) sets.String {
	ipSet := sets.NewString()
	for ip := range ipmap {
		ipSet.Insert(ip.String())
	}

	return ipSet
}
