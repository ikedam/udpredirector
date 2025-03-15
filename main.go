package main

import (
	"log"
	"net"
	"syscall"
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
	targetMAC := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55} // 例: 00:11:22:33:44:55
	interfaceName := "eth0"                                 // 使用するネットワークインターフェース名

	// ネットワークインターフェースを取得
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		log.Fatalf("Failed to get interface %s: %v", interfaceName, err)
	}

	// RAWソケットを作成
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(syscall.ETH_P_ALL))
	if err != nil {
		log.Fatalf("Failed to create raw socket: %v", err)
	}
	defer syscall.Close(fd)

	// ソケットをインターフェースにバインド
	addr := syscall.SockaddrLinklayer{
		Protocol: syscall.ETH_P_ALL,
		Ifindex:  iface.Index,
	}
	if err := syscall.Bind(fd, &addr); err != nil {
		log.Fatalf("Failed to bind raw socket: %v", err)
	}

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
		ethFrame := append(targetMAC, iface.HardwareAddr...) // 宛先MAC + 送信元MAC
		ethFrame = append(ethFrame, 0x08, 0x00)              // EtherType (IPv4)
		ethFrame = append(ethFrame, buffer[:n]...)           // ペイロード

		// RAWソケットで送信
		err = syscall.Sendto(fd, ethFrame, 0, &syscall.SockaddrLinklayer{
			Ifindex:  iface.Index,
			Protocol: syscall.ETH_P_ALL,
		})
		if err != nil {
			log.Printf("Error sending Ethernet frame: %v", err)
			continue
		}

		log.Printf("Forwarded packet to MAC %s", targetMAC)
	}
}
