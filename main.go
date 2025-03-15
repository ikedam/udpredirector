package main

import (
	"log"
	"net"
	"syscall"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "udpredirector [port] [mac] [interface]",
		Short: "UDP Redirector forwards UDP packets to a specific MAC address",
		Args:  cobra.ExactArgs(3), // 必須の位置引数を3つに設定
		Run: func(cmd *cobra.Command, args []string) {
			// 位置引数を取得
			port := args[0]
			targetMACAddr := args[1]
			interfaceName := args[2]

			// MACアドレスをパース
			targetMAC, err := net.ParseMAC(targetMACAddr)
			if err != nil {
				log.Fatalf("Invalid MAC address: %v", err)
			}

			// UDPの受信ポートを指定
			listenAddr := ":" + port
			conn, err := net.ListenPacket("udp", listenAddr)
			if err != nil {
				log.Fatalf("Failed to listen on UDP port: %v", err)
			}
			defer conn.Close()

			log.Printf("Listening for UDP packets on %s", listenAddr)

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
		},
	}

	// コマンドの実行
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
