package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go <server-address> <port>")
		return
	}

	address := net.JoinHostPort(os.Args[1], os.Args[2])
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Failed to connect to server at %s: %v\n", address, err)
		return
	}
	defer conn.Close()

	fmt.Printf("✅ Connected to distributed cache at %s\n", address)
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(">> ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			break
		}

		command := strings.TrimSpace(input)
		if command == "" {
			continue
		}

		if strings.EqualFold(command, "exit") {
			fmt.Println("👋 Exiting client...")
			break
		}

		// Send command to server
		_, err = conn.Write([]byte(command))
		if err != nil {
			fmt.Printf("Error sending command: %v\n", err)
			break
		}

		// Read response
		reply := make([]byte, 2048)
		n, err := conn.Read(reply)
		if err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			break
		}

		fmt.Println("<<", strings.TrimSpace(string(reply[:n])))
	}
}
