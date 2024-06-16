package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

const (
	host   = "localhost"
	port   = "5432"
	user   = "user"
	dbname = "chat"
)

type User struct {
	Name     string
	Password string
	Conn     *websocket.Conn
	Chatting bool

}

type Message struct {
	Type string
	Data interface{}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var NewUser = make(chan User, 1)
var DelUser = make(chan string, 1)
var OnlineUsers = make(map[string]User)
var exit = make(chan os.Signal, 1)

var db *sql.DB

func send(c *websocket.Conn, Type string, Data ...interface{}) {
	var err error
	if len(Data) == 0 {
		err = c.WriteJSON(Message{Type: Type})
	} else {
		err = c.WriteJSON(Message{Type: Type, Data: Data[0]})
	}
	if err != nil {
		c.Close()
		panic(err)
	}
}

func C(err error) { // check error
	if err != nil && err != sql.ErrNoRows {
		log.Println(err)
	}
}

func signals() { // handle GIGINT & SIGTERM
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)
	<-exit
	db.Close()
	log.Println("Bye!")
	for _, i := range OnlineUsers {
		send(i.Conn, "ServerStop")
	}
	os.Exit(0)
}

func online() { // auto refresh online users
	OnlineSet := make(map[string]bool)
	for {
		select {
		case user := <-NewUser:
			OnlineUsers[user.Name] = user
			OnlineSet[user.Name] = true
		case user := <-DelUser:
			delete(OnlineUsers, user)
			delete(OnlineSet, user)
		}
		var OnlineNames string
		for i := range OnlineSet {
			OnlineNames += i + " "
		}
		for _, i := range OnlineUsers {
			send(i.Conn, "OnlineUsers", OnlineNames)
		}
	}
}

func connect(w http.ResponseWriter, r *http.Request) { // handle connection
	var msg Message
	var user User
	var mate User

	conn, err := upgrader.Upgrade(w, r, nil)
	C(err)
	log.Println("Connect:", conn.RemoteAddr())
	defer log.Println("Disconnect:", conn.RemoteAddr())

	for {
		err = conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Crashed:", user.Name, "msg:", msg, "error:", err)
			send(conn, "Crash")
			DelUser <- user.Name
			if mate.Chatting {
				mate.Chatting = false
				send(mate.Conn, "MateClosed")
			}
			return
		}
		switch msg.Type {
		case "Login":
			var DbUser, DbPass string
			data := msg.Data.([]interface{})
			if OnlineUsers[data[0].(string)] != (User{}) {
				log.Println("Already online:", data[0])
				send(conn, "AlreadyOnline")
			} else {
				err = db.QueryRow("SELECT * FROM users WHERE username = $1", data[0].(string)).Scan(&DbUser, &DbPass)
				if err == sql.ErrNoRows {
					send(conn, "UserNotFound")
					log.Println("User not found:", data[0])
				} else {
					C(err)
					if data[1] == DbPass {
						user = User{Name: DbUser, Password: DbPass, Conn: conn}
						send(conn, "Logged")
						NewUser <- user
						log.Println("Login:", user.Name)
					} else {
						send(conn, "WrongPassword")
						log.Println("Wrong password:", data[0], data[1])
					}
				}
			}
		case "Register":
			var count int
			data := msg.Data.([]interface{})
			C(db.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1", data[0]).Scan(&count))
			switch count {
			case 0:
				_, err = db.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", data[0], data[1])
				C(err)
				user = User{Name: msg.Data.([]interface{})[0].(string), Password: data[1].(string), Conn: conn}
				send(conn, "Registered")
				NewUser <- user
				log.Println("Register:", user.Name)
			case 1:
				send(conn, "UserExist")
				log.Println("Already exists:")
			}
		case "Invite":
			var found bool
			mate, found = OnlineUsers[msg.Data.(string)]
			
			if !found {
				send(conn, "UserNotFound")
				log.Println(user.Name, "->", mate.Name+": Invite not found")
			} else if mate.Chatting {
				send(conn, "UserIsChatting")
				log.Println(user.Name, "->", mate.Name+": Invite chatting")
			} else if mate.Name == user.Name {
				send(conn, "InviteYourself")
				log.Println(user.Name, "->", mate.Name+": Invite himself")
			} else {
				send(mate.Conn, "InviteRequest", user.Name)
				send(conn, "InviteSent")
				log.Println(user.Name, "->", mate.Name+": Invite")
			}
		case "InviteCrush":
			mate = OnlineUsers[msg.Data.(string)]
			log.Println(user.Name, "->", mate.Name+": Accept")
			send(mate.Conn, "InviteAccept", user.Name)

			user.Chatting = true
			mate.Chatting = true
			OnlineUsers[mate.Name] = mate 

			var chat string
			var user1, user2 string
			if user.Name < mate.Name {
				user1 = user.Name
				user2 = mate.Name
			} else {
				user1 = mate.Name
				user2 = user.Name
			}
			err = db.QueryRow("SELECT chat FROM history WHERE user1 = $1 AND user2 = $2", user1, user2).Scan(&chat)
			C(err)
			if err == sql.ErrNoRows {
				_, err = db.Exec("INSERT INTO history VALUES ($1, $2, $3)", user1, user2, "")
				C(err)
			} else {
				send(conn, "History", chat)
				send(mate.Conn, "History", chat)
			}
		case "InvitePass":
			log.Println(user.Name, "->", mate.Name+": Refuse")
			send(OnlineUsers[msg.Data.(string)].Conn, "InviteRefuse", user.Name)
		case "Chat":
			send(mate.Conn, "Chat", msg.Data)
			log.Print(user.Name, " -> ", mate.Name, ": ", "\"", msg.Data, "\"")

			var user1, user2 string
			if user.Name < mate.Name {
				user1 = user.Name
				user2 = mate.Name
			} else {
				user1 = mate.Name
				user2 = user.Name
			}
			_, err = db.Exec("UPDATE history SET chat = chat || $3 WHERE user1 = $1 AND user2 = $2", user1, user2, user.Name + "> " + msg.Data.(string) + "\n")
			C(err)
		case "Ping":
			send(conn, "Pong")
		case "Close":
			if user.Name != "" {
				DelUser <- user.Name
				log.Println("Quit:", user.Name)
				if OnlineUsers[mate.Name].Chatting {
					mate.Chatting = false
					send(mate.Conn, "MateClosed")
				}
			}
			return
		default:
			log.Println("error msg type", msg.Type)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Println("Usage: ./server [port]")
		return
	}
	var err error
	psqlconn := "host=" + host + " port=" + port + " user=" + user + " dbname=" + dbname + " sslmode=disable"
	log.Println(psqlconn)
	db, err = sql.Open("postgres", psqlconn)
	if err != nil {
		log.Println("Database connection failed. Is postgresql running?")
		panic(err)
	}
	defer db.Close()

	C(db.Ping())
	log.Println("Database connected")

	go online()
	go signals()
	http.HandleFunc("/ws", connect)

	C(http.ListenAndServe(":" + os.Args[1], nil))
	log.Println("Server started on :" + os.Args[1])
}
