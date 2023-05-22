package chat

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"go_chat/trace"
)

type Room struct {

	// forward is a channel that holds incoming messages
	// that should be forwarded to the other clients.
	forward chan []byte

	// join is a channel for clients wishing to join the room.
	join chan *Client

	// leave is a channel for clients wishing to leave the room.
	leave chan *Client

	// clients holds all current clients in this room.
	clients map[*Client]bool

	// tracer will receive trace information of activity
	// in the room.
	Tracer trace.Tracer
}

// newRoom makes a new room that is ready to
// go.
func NewRoom() *Room {
	return &Room{
		forward: make(chan []byte),
		join:    make(chan *Client),
		leave:   make(chan *Client),
		clients: make(map[*Client]bool),
		Tracer:  trace.Off(),
	}
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.join:
			// joining
			r.clients[client] = true
			r.Tracer.Trace("New client joined")
		case client := <-r.leave:
			// leaving
			delete(r.clients, client)
			close(client.send)
			r.Tracer.Trace("Client left")
		case msg := <-r.forward:
			r.Tracer.Trace("Message received: ", string(msg))
			// forward message to all clients
			for client := range r.clients {
				client.send <- msg
				r.Tracer.Trace(" -- sent to client")
			}
		}
	}
}

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

var upgrader = &websocket.Upgrader{ReadBufferSize: socketBufferSize, WriteBufferSize: socketBufferSize}

func (r *Room) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Fatal("ServeHTTP:", err)
		return
	}
	client := &Client{
		socket: socket,
		send:   make(chan []byte, messageBufferSize),
		room:   r,
	}
	r.join <- client
	defer func() { r.leave <- client }()
	go client.write()
	client.read()
}
