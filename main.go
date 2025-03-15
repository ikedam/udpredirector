package main

import (
	"log"
	"net"

	"golang.org/x/net/ethernet"
)

func main() {
	// UDPの受信ポートを指定
	listenAddr := ":12345"
	conn, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP port: %v", err)
	}
	defer conn.Close()

	log.Printf("Listening for UDP packets on %s", listenAddr)

	// 転送先のMACアドレスを指定
	targetMAC := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55} // 例: 00:11:22:33:44:55
	interfaceName := "eth0"                                           // 使用するネットワークインターフェース名

	// ネットワークインターフェースを取得
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		log.Fatalf("Failed to get interface %s: %v", interfaceName, err)
	}

	// RAWソケットを作成
	rawConn, err := ethernet.ListenPacket(iface)
	if err != nil {
		log.Fatalf("Failed to create raw socket: %v", err)
	}
	defer rawConn.Close()

	buffer := make([]byte, 1500) // 最大パケットサイズ

	for {
		// UDPパケットを受信
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			log.Printf("Error reading UDP packet: %v", err)
			continue
		}

		log.Printf("Received %d bytes from %s", n, addr)

		// Ethernetフレームを構築
		ethFrame := &ethernet.Frame{
			Destination: targetMAC,
			Source:      iface.HardwareAddr,
			EtherType:   ethernet.EtherTypeIPv4,
			Payload:     buffer[:n],
		}

		// Ethernetフレームをシリアライズ
		ethData, err := ethFrame.MarshalBinary()
		if err != nil {
			log.Printf("Error serializing Ethernet frame: %v", err)
			continue
		}

		// RAWソケットで送信
		_, err = rawConn.WriteTo(ethData, &ethernet.Addr{HardwareAddr: targetMAC})
		if err != nil {
			log.Printf("Error sending Ethernet frame: %v", err)
			continue
		}

		log.Printf("Forwarded packet to MAC %s", targetMAC)
	}
}
