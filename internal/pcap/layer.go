package pcap

import (
	"errors"
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func createTCPLayerSYN(srcPort, dstPort uint16, seq uint32) *layers.TCP {
	return &layers.TCP{
		SrcPort:    layers.TCPPort(srcPort),
		DstPort:    layers.TCPPort(dstPort),
		Seq:        seq,
		DataOffset: 5,
		SYN:        true,
		Window:     65535,
		// Checksum:   0,
	}
}

func createTCPLayerSYNACK(srcPort, dstPort uint16, seq, ack uint32) *layers.TCP {
	return &layers.TCP{
		SrcPort:    layers.TCPPort(srcPort),
		DstPort:    layers.TCPPort(dstPort),
		Seq:        seq,
		Ack:        ack,
		DataOffset: 5,
		SYN:        true,
		ACK:        true,
		Window:     65535,
		// Checksum:   0,
	}
}

func createTCPLayerACK(srcPort, dstPort uint16, seq, ack uint32) *layers.TCP {
	return &layers.TCP{
		SrcPort:    layers.TCPPort(srcPort),
		DstPort:    layers.TCPPort(dstPort),
		Seq:        seq,
		Ack:        ack,
		DataOffset: 5,
		ACK:        true,
		Window:     65535,
		// Checksum:   0,
	}
}

func createTransportLayerTCP(srcPort, dstPort uint16, seq, ack uint32) *layers.TCP {
	return &layers.TCP{
		SrcPort:    layers.TCPPort(srcPort),
		DstPort:    layers.TCPPort(dstPort),
		Seq:        seq,
		Ack:        ack,
		DataOffset: 5,
		PSH:        true,
		ACK:        true,
		Window:     65535,
		// Checksum:   0,
	}
}

func createTransportLayerUDP(srcPort, dstPort uint16) *layers.UDP {
	return &layers.UDP{
		SrcPort: layers.UDPPort(srcPort),
		DstPort: layers.UDPPort(dstPort),
		// Length:    0,
		// Checksum:  0,
	}
}

func createNetworkLayerIPv4(srcIP, dstIP net.IP, id uint16, ttl uint8, transportLayer gopacket.TransportLayer) (*layers.IPv4, error) {
	if srcIP.To4() == nil || dstIP.To4() == nil {
		return nil, fmt.Errorf("create network layer: %w", fmt.Errorf("invalid ipv4 address %s", srcIP))
	}

	ipv4Layer := &layers.IPv4{
		Version: 4,
		IHL:     5,
		// Length:     0,
		Id:    id,
		Flags: layers.IPv4DontFragment,
		TTL:   ttl,
		// Protocol:   0,
		// Checksum:   0,
		SrcIP: srcIP,
		DstIP: dstIP,
	}

	// Protocol
	transportLayerType := transportLayer.LayerType()
	switch transportLayerType {
	case layers.LayerTypeTCP:
		ipv4Layer.Protocol = layers.IPProtocolTCP

		// Checksum of transport layer
		tcpLayer := transportLayer.(*layers.TCP)
		err := tcpLayer.SetNetworkLayerForChecksum(ipv4Layer)
		if err != nil {
			return nil, fmt.Errorf("create network layer: %w", err)
		}
	case layers.LayerTypeUDP:
		ipv4Layer.Protocol = layers.IPProtocolUDP

		// Checksum of transport layer
		udpLayer := transportLayer.(*layers.UDP)
		err := udpLayer.SetNetworkLayerForChecksum(ipv4Layer)
		if err != nil {
			return nil, fmt.Errorf("create network layer: %w", err)
		}
	default:
		return nil, fmt.Errorf("create network layer: %w", fmt.Errorf("transport layer type %s not support", transportLayerType))
	}

	return ipv4Layer, nil
}

func createNetworkLayerIPv6(srcIP, dstIP net.IP, transportLayer gopacket.TransportLayer) (*layers.IPv6, error) {
	if srcIP.To4() != nil || dstIP.To4() != nil {
		return nil, fmt.Errorf("create network layer: %w", fmt.Errorf("invalid ipv6 address %s", srcIP))
	}
	return nil, fmt.Errorf("create network layer: %w", errors.New("ipv6 not support"))
}

func createLinkLayerLoopback() *layers.Loopback {
	return &layers.Loopback{}
}

func createLinkLayerEthernet(srcMAC, dstMAC net.HardwareAddr, networkLayer gopacket.NetworkLayer) (*layers.Ethernet, error) {
	ethernetLayer := &layers.Ethernet{
		SrcMAC: srcMAC,
		DstMAC: dstMAC,
		// EthernetType: 0,
	}

	// Protocol
	networkLayerType := networkLayer.LayerType()
	switch networkLayerType {
	case layers.LayerTypeIPv4:
		ethernetLayer.EthernetType = layers.EthernetTypeIPv4
	case layers.LayerTypeIPv6:
		ethernetLayer.EthernetType = layers.EthernetTypeIPv6
	default:
		return nil, fmt.Errorf("create link layer: %w", fmt.Errorf("type %s not support", networkLayerType))
	}

	return ethernetLayer, nil
}

func serialize(layers ...gopacket.SerializableLayer) ([]byte, error) {
	// Recalculate checksum and length
	options := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	buffer := gopacket.NewSerializeBuffer()

	err := gopacket.SerializeLayers(buffer, options, layers...)
	if err != nil {
		return nil, fmt.Errorf("serialize: %w", err)
	}

	return buffer.Bytes(), nil
}

func serializeRaw(layers ...gopacket.SerializableLayer) ([]byte, error) {
	// Recalculate checksum and length
	options := gopacket.SerializeOptions{}
	buffer := gopacket.NewSerializeBuffer()

	err := gopacket.SerializeLayers(buffer, options, layers...)
	if err != nil {
		return nil, fmt.Errorf("serialize: %w", err)
	}

	return buffer.Bytes(), nil
}

type icmpv4Indicator struct {
	layer                 *layers.ICMPv4
	embIPv4Layer          *layers.IPv4
	embTransportLayer     gopacket.Layer
	embTransportLayerType gopacket.LayerType
}

func parseICMPv4Layer(layer *layers.ICMPv4) (*icmpv4Indicator, error) {
	var (
		embIPv4Layer          *layers.IPv4
		embTransportLayer     gopacket.Layer
		embTransportLayerType gopacket.LayerType
	)

	t := layer.TypeCode.Type()
	switch t {
	case layers.ICMPv4TypeEchoReply,
		layers.ICMPv4TypeEchoRequest,
		layers.ICMPv4TypeRouterAdvertisement,
		layers.ICMPv4TypeRouterSolicitation,
		layers.ICMPv4TypeTimestampRequest,
		layers.ICMPv4TypeTimestampReply,
		layers.ICMPv4TypeInfoRequest,
		layers.ICMPv4TypeInfoReply,
		layers.ICMPv4TypeAddressMaskRequest,
		layers.ICMPv4TypeAddressMaskReply:
		break
	case layers.ICMPv4TypeDestinationUnreachable,
		layers.ICMPv4TypeSourceQuench,
		layers.ICMPv4TypeRedirect,
		layers.ICMPv4TypeTimeExceeded,
		layers.ICMPv4TypeParameterProblem:
		// Parse IPv4 header and 8 bytes content
		packet := gopacket.NewPacket(layer.Payload, layers.LayerTypeIPv4, gopacket.Default)
		if len(packet.Layers()) <= 0 {
			return nil, fmt.Errorf("parse icmp v4 layer: %w", errors.New("missing network layer"))
		}
		if len(packet.Layers()) <= 1 {
			return nil, fmt.Errorf("parse icmp v4 layer: %w", errors.New("missing transport layer"))
		}

		networkLayer := packet.Layers()[0]
		if networkLayer.LayerType() != layers.LayerTypeIPv4 {
			return nil, fmt.Errorf("parse icmp v4 layer: %w", errors.New("network layer type not support"))
		}

		embIPv4Layer = networkLayer.(*layers.IPv4)
		if embIPv4Layer.Version != 4 {
			return nil, fmt.Errorf("parse icmp v4 layer: %w", fmt.Errorf("ip version %d not support", embIPv4Layer.Version))
		}

		embTransportLayer = packet.Layers()[1]
		embTransportLayerType = embTransportLayer.LayerType()
	default:
		return nil, fmt.Errorf("parse icmp v4 layer: %w", fmt.Errorf("type %d not support", t))
	}

	return &icmpv4Indicator{
		layer:                 layer,
		embIPv4Layer:          embIPv4Layer,
		embTransportLayer:     embTransportLayer,
		embTransportLayerType: embTransportLayerType,
	}, nil
}

func (indicator *icmpv4Indicator) newPureICMPv4Layer() *layers.ICMPv4 {
	return &layers.ICMPv4{
		TypeCode: indicator.layer.TypeCode,
		Id:       indicator.layer.Id,
		Seq:      indicator.layer.Seq,
	}
}

func (indicator *icmpv4Indicator) isQuery() bool {
	t := indicator.layer.TypeCode.Type()
	switch t {
	case layers.ICMPv4TypeEchoReply,
		layers.ICMPv4TypeEchoRequest,
		layers.ICMPv4TypeRouterAdvertisement,
		layers.ICMPv4TypeRouterSolicitation,
		layers.ICMPv4TypeTimestampRequest,
		layers.ICMPv4TypeTimestampReply,
		layers.ICMPv4TypeInfoRequest,
		layers.ICMPv4TypeInfoReply,
		layers.ICMPv4TypeAddressMaskRequest,
		layers.ICMPv4TypeAddressMaskReply:
		return true
	case layers.ICMPv4TypeDestinationUnreachable,
		layers.ICMPv4TypeSourceQuench,
		layers.ICMPv4TypeRedirect,
		layers.ICMPv4TypeTimeExceeded,
		layers.ICMPv4TypeParameterProblem:
		return false
	default:
		panic(fmt.Errorf("is query: %w", fmt.Errorf("type %d not support", t)))
	}
}

func (indicator *icmpv4Indicator) embSrcIP() net.IP {
	return indicator.embIPv4Layer.SrcIP
}

func (indicator *icmpv4Indicator) embDstIP() net.IP {
	return indicator.embIPv4Layer.DstIP
}

func (indicator *icmpv4Indicator) embTCPLayer() *layers.TCP {
	if indicator.embTransportLayerType == layers.LayerTypeTCP {
		return indicator.embTransportLayer.(*layers.TCP)
	}

	return nil
}

func (indicator *icmpv4Indicator) embUDPLayer() *layers.UDP {
	if indicator.embTransportLayerType == layers.LayerTypeUDP {
		return indicator.embTransportLayer.(*layers.UDP)
	}

	return nil
}

func (indicator *icmpv4Indicator) embICMPv4Layer() *layers.ICMPv4 {
	if indicator.embTransportLayerType == layers.LayerTypeICMPv4 {
		return indicator.embTransportLayer.(*layers.ICMPv4)
	}

	return nil
}

func (indicator *icmpv4Indicator) id() uint16 {
	return indicator.layer.Id
}

func (indicator *icmpv4Indicator) embId() uint16 {
	switch indicator.embTransportLayerType {
	case layers.LayerTypeICMPv4:
		return uint16(indicator.embICMPv4Layer().Id)
	default:
		panic(fmt.Errorf("emb id: %w", fmt.Errorf("type %s not support", indicator.embTransportLayerType)))
	}
}

func (indicator *icmpv4Indicator) embSrcPort() uint16 {
	switch indicator.embTransportLayerType {
	case layers.LayerTypeTCP:
		return uint16(indicator.embTCPLayer().SrcPort)
	case layers.LayerTypeUDP:
		return uint16(indicator.embUDPLayer().SrcPort)
	default:
		panic(fmt.Errorf("emb src port: %w", fmt.Errorf("type %s not support", indicator.embTransportLayerType)))
	}
}

func (indicator *icmpv4Indicator) embDstPort() uint16 {
	switch indicator.embTransportLayerType {
	case layers.LayerTypeTCP:
		return uint16(indicator.embTCPLayer().DstPort)
	case layers.LayerTypeUDP:
		return uint16(indicator.embUDPLayer().DstPort)
	default:
		panic(fmt.Errorf("emb dst port: %w", fmt.Errorf("type %s not support", indicator.embTransportLayerType)))
	}
}

func (indicator *icmpv4Indicator) isEmbQuery() bool {
	t := indicator.embICMPv4Layer().TypeCode.Type()
	switch t {
	case layers.ICMPv4TypeEchoReply,
		layers.ICMPv4TypeEchoRequest,
		layers.ICMPv4TypeRouterAdvertisement,
		layers.ICMPv4TypeRouterSolicitation,
		layers.ICMPv4TypeTimestampRequest,
		layers.ICMPv4TypeTimestampReply,
		layers.ICMPv4TypeInfoRequest,
		layers.ICMPv4TypeInfoReply,
		layers.ICMPv4TypeAddressMaskRequest,
		layers.ICMPv4TypeAddressMaskReply:
		return true
	case layers.ICMPv4TypeDestinationUnreachable,
		layers.ICMPv4TypeSourceQuench,
		layers.ICMPv4TypeRedirect,
		layers.ICMPv4TypeTimeExceeded,
		layers.ICMPv4TypeParameterProblem:
		return false
	default:
		panic(fmt.Errorf("is emb query: %w", fmt.Errorf("type %d not support", t)))
	}
}

func (indicator *icmpv4Indicator) natSrc() IPEndpoint {
	if indicator.isQuery() {
		panic(fmt.Errorf("src: %w", errors.New("icmpv4 query not support")))
	} else {
		// Flip source and destination
		switch indicator.embTransportLayerType {
		case layers.LayerTypeTCP, layers.LayerTypeUDP:
			return &IPPort{
				IP:   indicator.embDstIP(),
				Port: indicator.embDstPort(),
			}
		case layers.LayerTypeICMPv4:
			if indicator.isEmbQuery() {
				return &IPId{
					IP: indicator.embDstIP(),
					Id: indicator.embId(),
				}
			} else {
				return &IP{
					IP: indicator.embDstIP(),
				}
			}
		default:
			panic(fmt.Errorf("src: %w", fmt.Errorf("type %s not support", indicator.embTransportLayerType)))
		}
	}
}

func (indicator *icmpv4Indicator) natDst() IPEndpoint {
	if indicator.isQuery() {
		panic(fmt.Errorf("dst: %w", errors.New("icmpv4 query not support")))
	} else {
		// Flip source and destination
		switch indicator.embTransportLayerType {
		case layers.LayerTypeTCP, layers.LayerTypeUDP:
			return &IPPort{
				IP:   indicator.embSrcIP(),
				Port: indicator.embSrcPort(),
			}
		case layers.LayerTypeICMPv4:
			if indicator.isEmbQuery() {
				return &IPId{
					IP: indicator.embSrcIP(),
					Id: indicator.embId(),
				}
			} else {
				return &IP{
					IP: indicator.embSrcIP(),
				}
			}
		default:
			panic(fmt.Errorf("dst: %w", fmt.Errorf("type %s not support", indicator.embTransportLayerType)))
		}
	}
}