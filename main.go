package main

import (
	"net/http"

	"github.com/collinstommy/go-chat/components"
	"github.com/collinstommy/go-chat/template"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	e := echo.New()
	template.NewTemplateRenderer(e)

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// WebSocket endpoint
	e.GET("/ws", handleWebSocket)
	e.GET("/", func(c echo.Context) error {

		component := components.Index("Jon")

		return template.Html(c, component)
	})

	e.Logger.Fatal(e.Start(":1323"))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleWebSocket(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	// Register client
	client := &Client{
		conn: ws,
		send: make(chan []byte, 256),
	}
	clients.register <- client
	defer func() {
		clients.unregister <- client
	}()

	// Start goroutines for reading and writing messages
	go client.writePump()
	client.readPump()

	return nil
}

// Client represents a chat client
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Clients represents a collection of connected clients
type Clients struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

var clients = Clients{
	clients:    make(map[*Client]bool),
	broadcast:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
}

func (c *Client) readPump() {
	defer func() {
		clients.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		clients.broadcast <- message
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
				return
			}
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		}
	}
}

func init() {
	go clients.run()
}

func (clients *Clients) run() {
	for {
		select {
		case client := <-clients.register:
			clients.clients[client] = true
		case client := <-clients.unregister:
			if _, ok := clients.clients[client]; ok {
				delete(clients.clients, client)
				close(client.send)
			}
		case message := <-clients.broadcast:
			for client := range clients.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(clients.clients, client)
				}
			}
		}
	}
}
