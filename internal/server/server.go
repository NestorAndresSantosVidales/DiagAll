package server

import (
	"diagall/internal/engine"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

//go:embed ui/*
var uiFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client represents a connected WebSocket client.
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

var (
	clients    = make(map[*Client]bool)
	broadcast  = make(chan []byte)
	register   = make(chan *Client)
	unregister = make(chan *Client)
	mu         sync.Mutex
)

// StartServer starts the embedded web server.
func StartServer(port int) {
	// Serve static files
	uiAssets, _ := fs.Sub(uiFS, "ui")
	http.Handle("/", http.FileServer(http.FS(uiAssets)))

	// WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)

	// API endpoint to trigger profiles
	http.HandleFunc("/api/run", handleRunProfile)

	go handleMessages()

	fmt.Printf("Web UI started at http://localhost:%d\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	client := &Client{conn: conn, send: make(chan []byte, 256)}
	register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		unregister <- c
		c.conn.Close()
	}()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// Handle message
		handleClientMessage(c, message)
	}
}

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func handleMessages() {
	for {
		select {
		case client := <-register:
			mu.Lock()
			clients[client] = true
			mu.Unlock()
		case client := <-unregister:
			mu.Lock()
			if _, ok := clients[client]; ok {
				delete(clients, client)
				close(client.send)
			}
			mu.Unlock()
		case message := <-broadcast:
			mu.Lock()
			for client := range clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(clients, client)
				}
			}
			mu.Unlock()
		}
	}
}

func handleRunProfile(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	target := r.URL.Query().Get("target")

	if profile == "" || target == "" {
		http.Error(w, "Missing profile or target", http.StatusBadRequest)
		return
	}

	go runProfileAndStream(profile, target)
	w.Write([]byte("Job started"))
}

func runProfileAndStream(profileName, target string) {
	broadcast <- []byte(fmt.Sprintf("START:%s:%s", profileName, target))

	// Create a logger that broadcasts to WebSocket
	// Note: In a real app we might want to target specific clients,
	// but here we broadcast to all connected UIs.
	streamLog := func(s string) {
		broadcast <- []byte(s)
	}

	engine.RunProfile(profileName, target, streamLog)

	broadcast <- []byte("DONE")
}
