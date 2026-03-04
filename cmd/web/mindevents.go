package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type connection struct {
	token   string
	user_id *int
	conn    *websocket.Conn
	files   map[string]*mfile
	alias   int
}

var upgrader = websocket.Upgrader{}
var connections = map[string]*connection{}

var files = map[string]*mfile{}

type mfile struct {
	id   string
	subs map[string]*connection
}

func (f mfile) subscribe(conn *connection, token string) {
	f.subs[token] = conn
}

func (f mfile) unsubscribe(token string) {
	delete(f.subs, token)
}

func (f mfile) broadcast(msg interface{}, sender string) {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for _, conn := range f.subs {
		if sender != "" && sender == conn.token {
			continue
		}

		sendMsg(conn.conn, jsonMsg)
	}
}

func (f mfile) broadcastUsers() {
	users := []user{}

	i := 0
	for _, conn := range f.subs {
		if i == 3 {
			break
		}

		users = append(users, user{
			Token: conn.token,
			Alias: conn.alias,
		})

		i++
	}

	f.broadcast(usersMessage{
		Type:   "users",
		FileId: f.id,
		Data:   users,
		Total:  len(f.subs),
	}, "")
}

type clientMessage struct {
	Action string `json:"action"`
	FileId string `json:"fileId"`
	Data   string `json:"data"`
}

type serviceMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	Text string      `json:"text"`
}

type eventMessage struct {
	Type      string    `json:"type"`
	FileId    string    `json:"fileId"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

type user struct {
	Token   string `json:"token"`
	Name    string `json:"name"`
	User_id *int   `json:"id"`
	Alias   int    `json:"alias"`
}

type usersMessage struct {
	Type   string `json:"type"`
	FileId string `json:"fileId"`
	Data   []user `json:"data"`
	Total  int    `json:"total"`
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	connectionToken := uuid.NewString()
	connections[connectionToken] = &connection{
		token: connectionToken,
		conn:  conn,
		files: map[string]*mfile{},
		alias: rand.Intn(12),
	}

	reader(connections[connectionToken], connectionToken)
	defer removeConn(connectionToken)
}

func reader(conn *connection, connectionToken string) {
	for {
		messageType, p, err := conn.conn.ReadMessage()
		if err != nil {
			//log.Println(err)
			return
		}

		if messageType == websocket.TextMessage {
			readMindMsg(conn, connectionToken, p)
		}
	}
}

func sendMsg(conn *websocket.Conn, msg []byte) {
	if err := conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		log.Println("sendMsg", err)
	}
}

func removeConn(connectionToken string) {
	//log.Println("removeConn", connectionToken, connections[connectionToken])
	for _, file := range connections[connectionToken].files {
		file.unsubscribe(connectionToken)

		if len(file.subs) == 0 {
			//log.Println("remove 0", file, len(file.subs))
			delete(files, file.id)
		} else {
			//log.Println("remove", file, len(file.subs))
			file.broadcastUsers()
		}
	}

	delete(connections, connectionToken)
}
