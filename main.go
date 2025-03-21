package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func main() {
	var pidFile string

	var rootCmd = &cobra.Command{
		Use:   "udpredirector [port] [mac] [interface]",
		Short: "UDP Redirector forwards UDP packets to a specific MAC address",
		Args:  cobra.ExactArgs(3), // 必須の位置引数を3つに設定
		RunE: func(cmd *cobra.Command, args []string) error {
			// 位置引数を取得
			port := args[0]
			targetMACAddr := args[1]
			interfaceName := args[2]

			return startRedirector(port, targetMACAddr, interfaceName, pidFile)
		},
	}

	// フラグの追加
	rootCmd.Flags().StringVarP(&pidFile, "pidfile", "p", "", "Path to the PID file")

	// コマンドの実行
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %+v", err)
	}
}

func startRedirector(port, targetMACAddr, interfaceName, pidFile string) error {
	if pidFile != "" {
		// PIDファイルを作成
		if err := writePIDFile(pidFile); err != nil {
			return err
		}
		defer removePIDFile(pidFile)
	}

	// MACアドレスをパース
	targetMAC, err := net.ParseMAC(targetMACAddr)
	if err != nil {
		return fmt.Errorf("invalid MAC address: %w", err)
	}

	// UDPの受信ポートを指定
	listenAddr := ":" + port
	conn, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP port: %w", err)
	}
	defer conn.Close()

	log.Printf("Listening for UDP packets on %s", listenAddr)

	// ネットワークインターフェースを取得
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", interfaceName, err)
	}

	// RAWソケットを作成
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(syscall.ETH_P_ALL))
	if err != nil {
		return fmt.Errorf("failed to create raw socket: %w", err)
	}
	defer syscall.Close(fd)

	// ソケットをインターフェースにバインド
	addr := syscall.SockaddrLinklayer{
		Protocol: syscall.ETH_P_ALL,
		Ifindex:  iface.Index,
	}
	if err := syscall.Bind(fd, &addr); err != nil {
		return fmt.Errorf("failed to bind raw socket: %w", err)
	}

	// シグナル処理のセットアップ
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	// メイン処理を別のゴルーチンで実行
	done := make(chan error, 1)
	go func() {
		done <- loopRedirector(targetMAC, conn, iface, fd)
	}()

	select {
	case <-signalChan:
		break
	case err := <-done:
		return err
	}

	log.Println("Received termination signal, shutting down...")
	return nil
}

func loopRedirector(
	targetMAC net.HardwareAddr,
	conn net.PacketConn,
	iface *net.Interface,
	fd int,
) error {
	buffer := make([]byte, 1500) // 最大パケットサイズ

	for {
		// UDPパケットを受信
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Printf("UDP connection closed, shutting down...")
				break
			}
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

	return nil
}

func writePIDFile(pidFile string) error {
	pid := os.Getpid()
	file, err := os.Create(pidFile)
	if err != nil {
		return fmt.Errorf("failed to create PID file: %w", err)
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%d", pid)
	if err != nil {
		return fmt.Errorf("failed to write to PID file: %w", err)
	}

	log.Printf("PID file created at %s with PID %d", pidFile, pid)
	return nil
}

func removePIDFile(pidFile string) {
	err := os.Remove(pidFile)
	if err != nil {
		log.Printf("Failed to remove PID file: %v", err)
	} else {
		log.Printf("PID file %s removed", pidFile)
	}
}
