package main

import (
	"bufio"
	"context"
	"fmt"
	redis "github.com/go-redis/redis/v8"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

type User struct {
	ID   string
	Name string
}

var (
	clients      = make(map[net.Conn]*User) // Store client connections and their names
	redisClient  *redis.Client
	ctx          = context.Background()
	clientsMutex sync.Mutex
)

func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Redis server address
	})
}

func generateUserID(conn net.Conn) string {
	return conn.RemoteAddr().String() // Use the remote address as a unique ID
}

func main() {
	initRedis()

	listener, err := net.Listen("tcp", ":8080") // Start listening on port 8080
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	// Channel to handle interrupts and SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ch := make(chan string) // Channel to send messages

	go broadcastMessages(ch) // Start a goroutine to broadcast messages

	go func() {
		for sig := range sigChan {
			fmt.Printf("Received signal: %s\n", sig)
			flushRedisDB() // Flush Redis DB on termination
			os.Exit(0)     // Exit the application
		}
	}()

	for {
		conn, err := listener.Accept() // Accept new connection
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		fmt.Println("New connection accepted")
		go handleClient(conn, ch) // Start a new goroutine to handle this client
	}
}

func handleClient(conn net.Conn, ch chan<- string) {
	defer func() {
		conn.Close()
		removeClient(conn)
	}()

	reader := bufio.NewReader(conn)

	var user *User

	// Send initial prompt to the client
	sendPrompt(conn, "Enter your name: ")

	var name string
	for {
		name, _ = reader.ReadString('\n')
		name = strings.TrimSpace(name) // Clean up the input

		userID := generateUserID(conn)

		// Check if name already exists in Redis
		existingID, err := redisClient.HGet(ctx, "clients", name).Result()
		if err == nil && existingID != "" {
			// Inform the client that the name is taken and prompt again
			sendPrompt(conn, "Error: Name already taken, please choose another.")
			sendPrompt(conn, "Enter your name: ")
			continue
		}

		user = &User{
			ID:   userID,
			Name: name,
		}

		// Name is unique, store it
		addClientToRedis(user)
		break
	}

	clientsMutex.Lock()
	clients[conn] = user // Store the client's name
	clientsMutex.Unlock()
	sendPrompt(conn, fmt.Sprintf("Welcome, %s!-%s", name, user.ID))

	for {
		msg, err := reader.ReadString('\n') // Read message from client
		if err != nil {
			fmt.Println("Error reading from client:", err)
			return
		}
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}

		if strings.HasPrefix(msg, "/") {
			handleCommand(conn, msg)
			continue
		}

		// Prefix message with the user's name
		formattedMessage := fmt.Sprintf("%s-%s: %s\n", user.ID, name, msg)

		// Publish the message to Redis
		redisClient.Publish(ctx, "chat_messages", formattedMessage)
	}
}

func handleCommand(conn net.Conn, command string) {
	switch command {
	case "/online":
		listUsers(conn)
	case "/exit":
		sendPrompt(conn, "You have exited the chat.")
		removeClient(conn) // Remove client from the chat
		conn.Close()       // Close the connection
	default:
		sendPrompt(conn, "Unknown command.")
	}
}

func listUsers(conn net.Conn) {
	users, err := redisClient.HGetAll(ctx, "clients").Result()
	if err != nil {
		sendPrompt(conn, "Error fetching users from Redis: "+err.Error())
		return
	}

	if len(users) == 0 {
		sendPrompt(conn, "No users online.")
		return
	}

	var userList strings.Builder

	// Debugging output to ensure users map is correct
	fmt.Printf("Users from Redis: %v\n", users)

	for name, _ := range users {
		userList.WriteString(fmt.Sprintf("%s ", name))
	}

	// Ensure the entire message is sent properly
	sendPrompt(conn, "Online users: "+userList.String())

}

func sendPrompt(conn net.Conn, message string) {
	fmt.Fprintln(conn, message)
}

func broadcastMessages(ch <-chan string) {
	pubsub := redisClient.Subscribe(ctx, "chat_messages")
	defer pubsub.Close()

	for msg := range pubsub.Channel() {
		for client := range clients {
			fmt.Fprint(client, msg.Payload) // Send message to each client
		}
	}
}

func addClientToRedis(user *User) {
	err := redisClient.HSet(ctx, "clients", user.Name, user.ID).Err()
	if err != nil {
		fmt.Printf("Error adding client to Redis: %v\n", err)
	}
}
func removeClient(conn net.Conn) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	// Check if the client exists in the map
	if user, exists := clients[conn]; exists {
		if user != nil {
			removeClientFromRedis(user)
		}
		delete(clients, conn)
	}
}

func removeClientFromRedis(user *User) {
	redisClient.HDel(ctx, "clients", user.Name)
}

func flushRedisDB() {
	err := redisClient.FlushDB(ctx).Err()
	if err != nil {
		fmt.Printf("Error flushing Redis DB: %v\n", err)
	} else {
		fmt.Println("Redis DB flushed successfully.")
	}
}
