package main

import (
	"bufio"
	"fmt"
	"github.com/chzyer/readline"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var (
	clientID   string
	clientIDMu sync.Mutex
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8080") // Connect to the server
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer conn.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a new readline instance
	rl, err := readline.New(":$ ")
	if err != nil {
		fmt.Println("Error creating readline instance:", err)
		return
	}
	defer rl.Close()

	fmt.Print(":$ ") // Print the initial prompt

	// Handle server messages
	go func() {
		for {
			msg, err := bufio.NewReader(conn).ReadString('\n') // Read messages from the server
			if err != nil {
				if err.Error() == "EOF" || strings.Contains(err.Error(), "closed network connection") {
					fmt.Println("Server closed the connection.")
					os.Exit(0)
				}
				fmt.Println("Error reading from server:", err)
				return
			}
			msg = strings.TrimSpace(msg) // Clean up the input

			clientIDMu.Lock()
			if strings.HasPrefix(msg, "Welcome") {
				parts := strings.Split(msg, "-")
				clientID = parts[1]
				fmt.Println(parts[0]) // Print the welcome message
			} else if !strings.HasPrefix(msg, clientID+"-") {
				// Split message to get the content part
				parts := strings.Split(msg, "-")
				if len(parts) > 1 {
					fmt.Printf("-> %s\n", parts[1]) // Display messages from others with `->`
				} else {
					fmt.Println(msg)
				}
			}
			clientIDMu.Unlock()

			// Reprint the prompt after handling server messages
			fmt.Print(":$ ")
		}
	}()

	// Start a goroutine to handle user input and sending messages
	go func() {
		for {
			line, err := rl.Readline() // Read user input with readline
			if err != nil {
				if err.Error() == "Interrupt" || err.Error() == "EOF" {
					fmt.Println("\nInterrupt or EOF received. Exiting...")
					conn.Close()
					os.Exit(0)
				} else {
					fmt.Println("Error reading input:", err)
				}
				continue
			}
			line = strings.TrimSpace(line)
			if line == "/exit" {
				fmt.Fprintf(conn, line+"\n")
				conn.Close()
				os.Exit(0)
			} else if line != "" {
				// Send the message to the server
				fmt.Fprintf(conn, line+"\n")
			}
		}
	}()

	// Main loop for handling interrupts
	for {
		select {
		case <-sigChan: // Handle interrupt signal
			fmt.Println("\nInterrupt received. Exiting...")
			conn.Close()
			os.Exit(0)
		}
	}
}
