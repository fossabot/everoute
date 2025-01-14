package datapath

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/libOpenflow/openflow13"
	"github.com/contiv/ofnet/ofctrl"
)

//nolint
const (
	INPUT_TABLE               = 0
	CT_STATE_TABLE            = 1
	DIRECTION_SELECTION_TABLE = 10
	EGRESS_TIER0_TABLE        = 20
	EGRESS_TIER1_TABLE        = 25
	EGRESS_TIER2_TABLE        = 30
	INGRESS_TIER0_TABLE       = 50
	INGRESS_TIER1_TABLE       = 55
	INGRESS_TIER2_TABLE       = 60
	CT_COMMIT_TABLE           = 70
	SFC_POLICY_TABLE          = 80
	POLICY_FORWARDING_TABLE   = 90
)

type PolicyBridge struct {
	name            string
	OfSwitch        *ofctrl.OFSwitch
	datapathManager *DpManager

	inputTable              *ofctrl.Table
	ctStateTable            *ofctrl.Table
	directionSelectionTable *ofctrl.Table
	egressTier0PolicyTable  *ofctrl.Table
	egressTier1PolicyTable  *ofctrl.Table
	egressTier2PolicyTable  *ofctrl.Table
	ingressTier0PolicyTable *ofctrl.Table
	ingressTier1PolicyTable *ofctrl.Table
	ingressTier2PolicyTable *ofctrl.Table
	ctCommitTable           *ofctrl.Table
	sfcPolicyTable          *ofctrl.Table
	policyForwardingTable   *ofctrl.Table

	policySwitchStatusMutex sync.RWMutex
	isPolicySwitchConnected bool
}

func NewPolicyBridge(brName string, datapathManager *DpManager) *PolicyBridge {
	policyBridge := new(PolicyBridge)
	policyBridge.name = fmt.Sprintf("%s-policy", brName)
	policyBridge.datapathManager = datapathManager
	return policyBridge
}

func (p *PolicyBridge) SwitchConnected(sw *ofctrl.OFSwitch) {
	log.Infof("Switch %s connected", p.name)

	p.OfSwitch = sw

	p.policySwitchStatusMutex.Lock()
	p.isPolicySwitchConnected = true
	p.policySwitchStatusMutex.Unlock()
}

func (p *PolicyBridge) SwitchDisconnected(sw *ofctrl.OFSwitch) {
	log.Infof("Switch %s disconnected", p.name)

	p.policySwitchStatusMutex.Lock()
	p.isPolicySwitchConnected = false
	p.policySwitchStatusMutex.Unlock()

	p.OfSwitch = nil
}

func (p *PolicyBridge) IsSwitchConnected() bool {
	p.policySwitchStatusMutex.Lock()
	defer p.policySwitchStatusMutex.Unlock()

	return p.isPolicySwitchConnected
}

func (p *PolicyBridge) WaitForSwitchConnection() {
	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		p.policySwitchStatusMutex.Lock()
		if p.isPolicySwitchConnected {
			p.policySwitchStatusMutex.Unlock()
			return
		}
		p.policySwitchStatusMutex.Unlock()
	}

	log.Fatalf("OVS switch %s Failed to connect", p.name)
}

func (p *PolicyBridge) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
}

func (p *PolicyBridge) MultipartReply(sw *ofctrl.OFSwitch, rep *openflow13.MultipartReply) {
}

func (p *PolicyBridge) BridgeInit() {
	sw := p.OfSwitch

	p.inputTable = sw.DefaultTable()
	p.ctStateTable, _ = sw.NewTable(CT_STATE_TABLE)
	p.directionSelectionTable, _ = sw.NewTable(DIRECTION_SELECTION_TABLE)
	p.ingressTier0PolicyTable, _ = sw.NewTable(INGRESS_TIER0_TABLE)
	p.ingressTier1PolicyTable, _ = sw.NewTable(INGRESS_TIER1_TABLE)
	p.ingressTier2PolicyTable, _ = sw.NewTable(INGRESS_TIER2_TABLE)
	p.egressTier0PolicyTable, _ = sw.NewTable(EGRESS_TIER0_TABLE)
	p.egressTier1PolicyTable, _ = sw.NewTable(EGRESS_TIER1_TABLE)
	p.egressTier2PolicyTable, _ = sw.NewTable(EGRESS_TIER2_TABLE)
	p.ctCommitTable, _ = sw.NewTable(CT_COMMIT_TABLE)
	p.sfcPolicyTable, _ = sw.NewTable(SFC_POLICY_TABLE)
	p.policyForwardingTable, _ = sw.NewTable(POLICY_FORWARDING_TABLE)

	if err := p.initInputTable(sw); err != nil {
		log.Fatalf("Failed to init inputTable, error: %v", err)
	}
	if err := p.initCTFlow(sw); err != nil {
		log.Fatalf("Failed to init ct table, error: %v", err)
	}
	if err := p.initDirectionSelectionTable(); err != nil {
		log.Fatalf("Failed to init directionSelection table, error: %v", err)
	}
	if err := p.initPolicyTable(); err != nil {
		log.Fatalf("Failed to init policy table, error: %v", err)
	}
	if err := p.initPolicyForwardingTable(sw); err != nil {
		log.Fatalf("Failed to init policy forwarding table, error: %v", err)
	}
}

func (p *PolicyBridge) initDirectionSelectionTable() error {
	fromLocalToEgressFlow, _ := p.directionSelectionTable.NewFlow(ofctrl.FlowMatch{
		Priority:  MID_MATCH_FLOW_PRIORITY,
		InputPort: uint32(POLICY_TO_LOCAL_PORT),
	})
	if err := fromLocalToEgressFlow.Next(p.egressTier0PolicyTable); err != nil {
		return fmt.Errorf("failed to install from local to egress flow, error: %v", err)
	}
	fromUpstreamToIngressFlow, _ := p.directionSelectionTable.NewFlow(ofctrl.FlowMatch{
		Priority:  MID_MATCH_FLOW_PRIORITY,
		InputPort: uint32(POLICY_TO_CLS_PORT),
	})
	if err := fromUpstreamToIngressFlow.Next(p.ingressTier0PolicyTable); err != nil {
		return fmt.Errorf("failed to install from upstream to ingress flow, error: %v", err)
	}

	return nil
}

func (p *PolicyBridge) initInputTable(sw *ofctrl.OFSwitch) error {
	var ctStateTableID uint8 = CT_STATE_TABLE
	var policyConntrackZone uint16 = 65520
	ctAction := ofctrl.NewConntrackAction(false, false, &ctStateTableID, &policyConntrackZone)
	inputIPRedirectFlow, _ := p.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  HIGH_MATCH_FLOW_PRIORITY,
		Ethertype: PROTOCOL_IP,
	})
	_ = inputIPRedirectFlow.SetConntrack(ctAction)

	// Table 0, from local bridge flow
	inputFromLocalFlow, _ := p.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  HIGH_MATCH_FLOW_PRIORITY,
		InputPort: uint32(POLICY_TO_LOCAL_PORT),
	})
	outputPort, _ := sw.OutputPort(POLICY_TO_CLS_PORT)
	if err := inputFromLocalFlow.Next(outputPort); err != nil {
		return fmt.Errorf("failed to install input from local flow, error: %v", err)
	}

	// Table 0, from cls bridge flow
	inputFromUpstreamFlow, _ := p.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  HIGH_MATCH_FLOW_PRIORITY,
		InputPort: uint32(POLICY_TO_CLS_PORT),
	})
	outputPort, _ = sw.OutputPort(POLICY_TO_LOCAL_PORT)
	if err := inputFromUpstreamFlow.Next(outputPort); err != nil {
		return fmt.Errorf("failed to install input from upstream flow, error: %v", err)
	}

	// Table 0, default flow
	inputDefaultFlow, _ := p.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := inputDefaultFlow.Next(sw.DropAction()); err != nil {
		return fmt.Errorf("failed to install input default flow, error: %v", err)
	}

	return nil
}

func (p *PolicyBridge) initCTFlow(sw *ofctrl.OFSwitch) error {
	var policyConntrackZone uint16 = 65520
	// Table 1, ctState table, est state flow
	// FIXME. should add ctEst flow and ctInv flow with same priority. With different, it have no side effect to flow intent.
	ctEstState := openflow13.NewCTStates()
	ctEstState.UnsetNew()
	ctEstState.SetEst()
	ctStateFlow, _ := p.ctStateTable.NewFlow(ofctrl.FlowMatch{
		Priority: MID_MATCH_FLOW_PRIORITY + FLOW_MATCH_OFFSET,
		CtStates: ctEstState,
	})
	if err := ctStateFlow.Next(p.ctCommitTable); err != nil {
		return fmt.Errorf("failed to install ct est state flow, error: %v", err)
	}

	// Table 1, ctState table, invalid state flow
	ctInvState := openflow13.NewCTStates()
	ctInvState.SetInv()
	ctInvState.SetTrk()
	ctInvFlow, _ := p.ctStateTable.NewFlow(ofctrl.FlowMatch{
		Priority: MID_MATCH_FLOW_PRIORITY,
		CtStates: ctInvState,
	})
	if err := ctInvFlow.Next(sw.DropAction()); err != nil {
		return fmt.Errorf("failed to install ct invalid state flow, error: %v", err)
	}

	// Table 1. default flow
	ctStateDefaultFlow, _ := p.ctStateTable.NewFlow(ofctrl.FlowMatch{
		Priority:  DEFAULT_FLOW_MISS_PRIORITY,
		Ethertype: PROTOCOL_IP,
	})
	if err := ctStateDefaultFlow.Next(p.directionSelectionTable); err != nil {
		log.Fatalf("failed to install ct state default flow, error: %v", err)
	}

	// Table 70 conntrack commit table
	ctTrkState := openflow13.NewCTStates()
	ctTrkState.SetNew()
	ctTrkState.SetTrk()
	ctCommitFlow, _ := p.ctCommitTable.NewFlow(ofctrl.FlowMatch{
		Priority:  MID_MATCH_FLOW_PRIORITY,
		Ethertype: PROTOCOL_IP,
		CtStates:  ctTrkState,
	})
	var sfcPolicyTable uint8 = SFC_POLICY_TABLE
	ctCommitAction := ofctrl.NewConntrackAction(true, false, &sfcPolicyTable, &policyConntrackZone)
	_ = ctCommitFlow.SetConntrack(ctCommitAction)

	ctCommitTableDefaultFlow, _ := p.ctCommitTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := ctCommitTableDefaultFlow.Next(p.sfcPolicyTable); err != nil {
		return fmt.Errorf("failed to install ct commit flow, error: %v", err)
	}

	return nil
}

func (p *PolicyBridge) initPolicyTable() error {
	// egress policy table
	egressTier1DefaultFlow, _ := p.egressTier0PolicyTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := egressTier1DefaultFlow.Next(p.egressTier1PolicyTable); err != nil {
		return fmt.Errorf("failed to install egress tier1 default flow, error: %v", err)
	}
	egressTier2DefaultFlow, _ := p.egressTier1PolicyTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := egressTier2DefaultFlow.Next(p.egressTier2PolicyTable); err != nil {
		return fmt.Errorf("failed to install egress tier2 default flow, error: %v", err)
	}
	egressTier3DefaultFlow, _ := p.egressTier2PolicyTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := egressTier3DefaultFlow.Next(p.ctCommitTable); err != nil {
		return fmt.Errorf("failed to install egress tier3 default flow, error: %v", err)
	}

	// ingress policy table
	ingressTier1DefaultFlow, _ := p.ingressTier0PolicyTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := ingressTier1DefaultFlow.Next(p.ingressTier1PolicyTable); err != nil {
		return fmt.Errorf("failed to install ingress tier1 default flow, error: %v", err)
	}
	ingressTier2DefaultFlow, _ := p.ingressTier1PolicyTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := ingressTier2DefaultFlow.Next(p.ingressTier2PolicyTable); err != nil {
		return fmt.Errorf("failed to install ingress tier2 default flow, error: %v", err)
	}
	ingressTier3DefaultFlow, _ := p.ingressTier2PolicyTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := ingressTier3DefaultFlow.Next(p.ctCommitTable); err != nil {
		return fmt.Errorf("failed to install ingress tier3 default flow, error: %v", err)
	}

	// sfc policy table
	sfcPolicyTableDefaultFlow, _ := p.sfcPolicyTable.NewFlow(ofctrl.FlowMatch{
		Priority: DEFAULT_FLOW_MISS_PRIORITY,
	})
	if err := sfcPolicyTableDefaultFlow.Next(p.policyForwardingTable); err != nil {
		return fmt.Errorf("failed to install sfc policy table default flow, error: %v", err)
	}

	return nil
}

func (p *PolicyBridge) initPolicyForwardingTable(sw *ofctrl.OFSwitch) error {
	// policy forwarding table
	fromLocalOutputFlow, _ := p.policyForwardingTable.NewFlow(ofctrl.FlowMatch{
		Priority:  NORMAL_MATCH_FLOW_PRIORITY,
		InputPort: uint32(POLICY_TO_LOCAL_PORT),
		Regs: []*ofctrl.NXRegister{
			{
				RegID: 6,
				Data:  0,
				Range: openflow13.NewNXRange(0, 15),
			},
		},
	})
	outputPort, _ := sw.OutputPort(POLICY_TO_CLS_PORT)
	if err := fromLocalOutputFlow.Next(outputPort); err != nil {
		return fmt.Errorf("failed to install from local output flow, error: %v", err)
	}

	fromUpstreamOuputFlow, _ := p.policyForwardingTable.NewFlow(ofctrl.FlowMatch{
		Priority:  NORMAL_MATCH_FLOW_PRIORITY,
		InputPort: uint32(POLICY_TO_CLS_PORT),
		Regs: []*ofctrl.NXRegister{
			{
				RegID: 6,
				Data:  0,
				Range: openflow13.NewNXRange(0, 15),
			},
		},
	})
	outputPort, _ = sw.OutputPort(POLICY_TO_LOCAL_PORT)
	if err := fromUpstreamOuputFlow.Next(outputPort); err != nil {
		return fmt.Errorf("failed to install from upstream output flow, error: %v", err)
	}

	return nil
}

func (p *PolicyBridge) BridgeReset() {
}

func (p *PolicyBridge) AddLocalEndpoint(endpoint *Endpoint) error {
	return nil
}

func (p *PolicyBridge) RemoveLocalEndpoint(endpoint *Endpoint) error {
	return nil
}

func (p *PolicyBridge) GetTierTable(direction uint8, tier uint8) (*ofctrl.Table, *ofctrl.Table, error) {
	var policyTable, nextTable *ofctrl.Table
	// POLICY_TIER0 for endpoint isolation policy:
	// 1) high priority rule is whitelist for support forensic policyrule, thus packet that match
	//    that rules should passthrough other policy tier ---- send to ctCommitTable;
	// 2) low priority rule is blacklist for support general isolation policyrule.
	switch direction {
	case POLICY_DIRECTION_OUT:
		switch tier {
		case POLICY_TIER0:
			policyTable = p.egressTier0PolicyTable
			nextTable = p.egressTier1PolicyTable
		case POLICY_TIER1:
			policyTable = p.egressTier1PolicyTable
			nextTable = p.ctCommitTable
		case POLICY_TIER2:
			policyTable = p.egressTier2PolicyTable
			nextTable = p.ctCommitTable
		default:
			return nil, nil, errors.New("unknow policy tier")
		}
	case POLICY_DIRECTION_IN:
		switch tier {
		case POLICY_TIER0:
			policyTable = p.ingressTier0PolicyTable
			nextTable = p.ingressTier1PolicyTable
		case POLICY_TIER1:
			policyTable = p.ingressTier1PolicyTable
			nextTable = p.ctCommitTable
		case POLICY_TIER2:
			policyTable = p.ingressTier2PolicyTable
			nextTable = p.ctCommitTable
		default:
			return nil, nil, errors.New("unknow policy tier")
		}
	}

	return policyTable, nextTable, nil
}

func (p *PolicyBridge) AddMicroSegmentRule(rule *EveroutePolicyRule, direction uint8, tier uint8) (*FlowEntry, error) {
	var ipDa *net.IP = nil
	var ipDaMask *net.IP = nil
	var ipSa *net.IP = nil
	var ipSaMask *net.IP = nil
	var err error

	// make sure switch is connected
	if !p.IsSwitchConnected() {
		p.WaitForSwitchConnection()
	}

	// Different tier have different nextTable select strategy:
	policyTable, nextTable, e := p.GetTierTable(direction, tier)
	if e != nil {
		log.Errorf("Failed to get policy table tier %v", tier)
		return nil, errors.New("failed get policy table")
	}

	// Parse dst ip
	if rule.DstIPAddr != "" {
		ipDa, ipDaMask, err = ParseIPAddrMaskString(rule.DstIPAddr)
		if err != nil {
			log.Errorf("Failed to parse dst ip %s. Err: %v", rule.DstIPAddr, err)
			return nil, err
		}
	}

	// parse src ip
	if rule.SrcIPAddr != "" {
		ipSa, ipSaMask, err = ParseIPAddrMaskString(rule.SrcIPAddr)
		if err != nil {
			log.Errorf("Failed to parse src ip %s. Err: %v", rule.SrcIPAddr, err)
			return nil, err
		}
	}

	// Install the rule in policy table
	ruleFlow, err := policyTable.NewFlow(ofctrl.FlowMatch{
		Priority:       uint16(rule.Priority),
		Ethertype:      PROTOCOL_IP,
		IpDa:           ipDa,
		IpDaMask:       ipDaMask,
		IpSa:           ipSa,
		IpSaMask:       ipSaMask,
		IpProto:        rule.IPProtocol,
		TcpSrcPort:     rule.SrcPort,
		TcpSrcPortMask: rule.SrcPortMask,
		TcpDstPort:     rule.DstPort,
		TcpDstPortMask: rule.DstPortMask,
		UdpSrcPort:     rule.SrcPort,
		UdpSrcPortMask: rule.SrcPortMask,
		UdpDstPort:     rule.DstPort,
		UdpDstPortMask: rule.DstPortMask,
	})
	if err != nil {
		log.Errorf("Failed to add flow for rule {%v}. Err: %v", rule, err)
		return nil, err
	}

	switch rule.Action {
	case "allow":
		err = ruleFlow.Next(nextTable)
		if err != nil {
			log.Errorf("Failed to install flow {%+v}. Err: %v", ruleFlow, err)
			return nil, err
		}
	case "deny":
		// Point it to next table
		err = ruleFlow.Next(p.OfSwitch.DropAction())
		if err != nil {
			log.Errorf("Failed to install flow {%+v}. Err: %v", ruleFlow, err)
			return nil, err
		}
	default:
		log.Errorf("Unknown action in rule {%+v}", rule)
		return nil, errors.New("unknown action in rule")
	}
	return &FlowEntry{
		Table:    policyTable,
		Priority: ruleFlow.Match.Priority,
		FlowID:   ruleFlow.FlowID,
	}, nil
}

func (p *PolicyBridge) RemoveMicroSegmentRule(rule *EveroutePolicyRule) error {
	return nil
}

func (p *PolicyBridge) AddVNFInstance() error {
	return nil
}

func (p *PolicyBridge) RemoveVNFInstance() error {
	return nil
}

func (p *PolicyBridge) AddSFCRule() error {
	return nil
}

func (p *PolicyBridge) RemoveSFCRule() error {
	return nil
}

func (p *PolicyBridge) BridgeInitCNI() {

}
