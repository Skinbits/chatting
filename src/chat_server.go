package main 

/*
    ---------     ws      ------------    talk
   | client1 | <-------> | Conenction | <------
    ---------             ------------    send |
                                               v
                                            --------      talk      ---------------        ws        ----------
                                           |  HUB   |<-----------> |   Conenction  | <------------> | SupportA |
                                            --------      send      ---------------                  ----------
                                               ^
    ---------             ------------    send |
   | client2 | <-------> | Conenction | <------
    ---------     ws      ------------    talk

   from:to:message

   client1:supportA:this is a message

   CID:SID:message

*/
import (
	"fmt"
	"code.google.com/p/go.net/websocket"
	// "io"
	"net/http"
	"html/template"
	"os"
	"strings"
	"strconv"
)

type ChatFrameVariables struct {
	Host string
	Port int
	Name string
}

type Connection struct {
	id int
	name string
	ws *websocket.Conn
	send chan string
}

type ChatHub struct {
	id int
	connections map[int]*Connection
	talk chan Message
	register chan *Connection
	unregister chan *Connection
}

type Message struct {
	c Connection
	message string
}

var (
	pwd, _ = os.Getwd()
	Server = &ChatFrameVariables{Host: "localhost", Port: 12345}
	hub = ChatHub{
	  id: 0,
		talk: make(chan Message),
		register: make(chan *Connection),
		unregister: make(chan *Connection),
		connections: make(map[int]*Connection),
	}
)

func (h *ChatHub) sendMessage(message string) {
	comma := strings.Index(message, ":")
	if comma == -1 {
		fmt.Printf("Invalid message. No from: %s\n", message)
		return;
	}
	message = message[comma:]
	comma = strings.Index(message, ":")
	if comma == -1 {
		fmt.Printf("Invalid message. No to: %s\n", message)
		return;
	}
	to,_ := strconv.Atoi(message[0:comma])
	message = message[comma:]
	c:=h.connections[to]
	if c == nil {
		fmt.Printf("Invalid message. Invalid to: %s\n", message)
		return;
	}
	select {
		case c.send <- message:
	  default:
	  	delete(h.connections, c.id)
	  	close(c.send)
	  	go c.ws.Close()
	}
}

func (h *ChatHub) run() {
	fmt.Printf("Hub starting\n")
	for {
		select {
		case c:= <-h.register:
			h.id++
			c.id = h.id
			fmt.Printf("New client with id %v\n", c.id)
			h.connections[c.id] = c
		case c:= <-h.unregister:
			delete(h.connections, c.id)
			close(c.send)
		case message:= <-h.talk:
			final := fmt.Sprintf("%s: %s", message.c.name, message.message)
			for _, c := range h.connections {
				if c.id != message.c.id {
					c.send <- final
				}
			}
		}
	}
}

func (c *Connection) reader() {
	fmt.Printf("Starting reader for %v\n", c.id)
	for {
		var message string
		err := websocket.Message.Receive(c.ws, &message)
		fmt.Printf("New message from client %v: %v\n", c.id, message)
		if err != nil {
			fmt.Printf("There was a error in client chat: %v\n", err.Error())
			break;
		}
		hub.talk <- Message{c: *c, message:message}
	}
	c.ws.Close()
}

func (c *Connection) writer() {
	for message := range c.send {
		fmt.Printf("New message to client %v: %v\n", c.id, message)
		err := websocket.Message.Send(c.ws, message)
		if err != nil {
			fmt.Printf("There was a error in client chat: %v", err.Error())
			break;
		}
	}
	c.ws.Close()
}

func wsHandler(ws *websocket.Conn) {
	fmt.Printf("New websocket connection\n")
	c := &Connection{
		id: 0, 
		name: "unkown", 
		ws: ws, 
		send: make(chan string, 256),
	}
	r := ws.Request()
	r.ParseForm()
	fmt.Printf("Params: \n")
	for key, value := range r.Form {
		fmt.Printf("%s -> %v\n", key, value)
	}
	c.name=r.Form["name"][0]
	if c.name == "" {
		c.name = "Client"
	}
	fmt.Printf("New websocket connection from %s\n", c.name)
	hub.register <- c
	defer func() { hub.unregister <- c}()
	go c.writer()
	c.reader()
}

func RootPage(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Calling root page\n")
	RootTemp := template.Must(template.ParseFiles( pwd + "/html/chat_client.html"))
	err := RootTemp.Execute(w, pwd)
	if err != nil {
		fmt.Printf("Error displaying root page")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func ChatFrameTemplate(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Calling chat frame\n")
	r.ParseForm()
	fmt.Printf("Params: \n")
	for key, value := range r.Form {
		fmt.Printf("%s -> %v\n", key, value)
	}
	ChatTemplate := template.Must(template.ParseFiles(pwd + "/html/client_frame.html"))
	data := &ChatFrameVariables{Host: Server.Host, Port: Server.Port, Name: r.Form["name"][0]}
	err := ChatTemplate.Execute(w, data)
	if err != nil {
		fmt.Printf("Error in chat frame: %v\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	go hub.run()
	fmt.Printf("Starting server\n")
  http.Handle("/ws", websocket.Handler(wsHandler))
  http.HandleFunc("/client_frame.html", ChatFrameTemplate)
  http.HandleFunc("/", RootPage)

  err := http.ListenAndServe(fmt.Sprintf("%s:%d", Server.Host, Server.Port) , nil)
  if err != nil {
      panic("ListenAndServe: " + err.Error())
  }
}