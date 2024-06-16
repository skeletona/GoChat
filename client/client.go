package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type string
	Data interface{}
}

type User struct {
	Name     string
	Password string
	Conn     *websocket.Conn
	Chatting bool
}

var input = make(chan string, 1)
var exit = make(chan os.Signal, 1)
var chatting = make(chan bool, 1)
var message = make(chan Message, 1)

var conn *websocket.Conn
var mate string
var OnlineUsers string
var PS1 = "GoChat> "
var cls = "^<ESC^>[2K\r" // clear line

func ping(conn *websocket.Conn) {
	for {
		if conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5 * time.Second)) != nil {
			fmt.Print("Connection timeout\n")
			exit <- syscall.SIGQUIT
		}
		time.Sleep(3 * time.Minute)
	}
}

func C(err error) {
	if err != nil {
		panic(err)
	}
}

func read(conn *websocket.Conn) {
	var msg Message
	for {
		if conn.ReadJSON(&msg) != nil {
			fmt.Print(cls, "Error reading from server: ", msg, "\n")
			exit <- syscall.SIGQUIT
		}
		switch msg.Type {
		case "ServerStop":
			fmt.Print(cls, "Server stopped by admin.\n")
			exit <- syscall.SIGQUIT
		case "MateClosed":
			fmt.Println("\nYour mate exited chat")
			exit <- syscall.SIGQUIT
		case "Crash":
			fmt.Println("\nYour connection crashed.")
			exit <- syscall.SIGQUIT
		default:
			message <- msg
		}
	}
}

func send(Type string, Data ...interface{}) {
	var err error
	if len(Data) == 0 {
		err = conn.WriteJSON(Message{Type: Type})
	} else {
		err = conn.WriteJSON(Message{Type: Type, Data: Data[0]})
	}
	if err != nil {
		fmt.Print("Error sending to server.", err, "\n")
		exit <- syscall.SIGQUIT
	}
}

func print(s any) {
	fmt.Print(cls, s, "\n", PS1)
}

func signals() {
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	call := <-exit
	send("Close")
	switch call {
	case syscall.SIGQUIT:
		fmt.Print(cls, "Bye!\n")
	default:
		fmt.Print(cls, "\nBye!\n")
	}
	os.Exit(0)
}

func ReadInput() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		switch text {
		case "h", "help":
			print("Short\tLong\n\nLogin:\nl\tlogin\nr\tregister\nInvite:\no\tonline\ni\tinvite\nAlways:\ne\texit\n")
		case "e", "exit", "quit":
			exit <- syscall.SIGQUIT
		case "":
			fmt.Print(PS1)
		default:
			input <- text
		}
	}
	C(scanner.Err())
}

func login(wg *sync.WaitGroup) bool {
	defer wg.Done()
	inp := <-input
	switch inp {
	case "l", "login", "r", "reg", "register":
		fmt.Print("Write your name: ")
		name := <-input
		fmt.Print("Write your password: ")
		pass := <-input
		fmt.Print("\033[1A")
		if inp == "l" || inp == "login" {
			send("Login", []string{name, pass})
		} else {
			send("Register", []string{name, pass})
		}
		msg := <-message
		switch msg.Type {
		case "Logged":
			PS1 = "GoChat " + name + "> "
			return false
		case "UserNotFound":
			print("User not found")
		case "WrongPassword":
			print("Wrong password")
		case "AlreadyOnline":
			print("User already online")
		case "Registered":
			PS1 = "GoChat " + name + "> "
			return false
		case "UserExist":
			print("User already exists.")
		default:
			print("error msg type " + msg.Type)
		}
	case "i", "invite", "o", "online":
		print("You're not logged in")
	default:
		print("No such command")
	}
	return true
}

func dialup(wg *sync.WaitGroup) { // enter chat with another user
	defer wg.Done()
	var stop_input = make(chan bool)

	go func() { // read
		for msg := range message {
			switch msg.Type {
			case "InviteSent":
				fmt.Print("Invite sent\n", PS1)
			case "InviteRefuse":
				print(msg.Data.(string) + " refused, you loser")
			case "UserNotFound":
				print("User not found")
			case "UserIsChatting":
				print("User is chatting")
			case "InviteRequest":
				fmt.Print(cls, msg.Data.(string)+" invited you. Accept? (Y/N) ")
				stop_input <- true
				inp := <-input
				switch inp {
				case "Y", "y", "yes", "Yes":
					send("InviteCrush", msg.Data)
					mate = msg.Data.(string)
					chatting <- true
					wg.Add(1)
					go chat(wg)
					return
				case "N", "n", "no", "No":
					send("InvitePass", msg.Data)
					fmt.Print(PS1)
				}
			case "InviteAccept":
				fmt.Print(cls, msg.Data, " accepted\n")
				mate = msg.Data.(string)
				chatting <- true
				wg.Add(1)
				go chat(wg)
				return
			case "InviteYourself":
				print("You cannot invite yourself")
			case "OnlineUsers":
				OnlineUsers = msg.Data.(string)
				print("Online users refresh: " + OnlineUsers)
			default:
				print("error msg type " + msg.Type)
			}
		}
	}()

	for { // send
		select {
		case tmp := <-input:
			inp := strings.Split(tmp, " ")
			switch inp[0] {
			case "i", "invite":
				if len(inp) < 2 {
					print("usage: invite [user]")
				} else {
					send("Invite", inp[1])
				}
			case "r", "register", "l", "login":
				print("You're already logged in")
			case "o", "online":
				fmt.Print(OnlineUsers, "\n", PS1)
			default:
				fmt.Print("No such command\n", PS1)
			}
		case <-stop_input:
		case <-chatting:
			return
		}
	}
}

func chat(wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Print(cls)
	go func() { // read
		for msg := range message {
			switch msg.Type {
			case "Chat":
				fmt.Println("\n" + mate+">", msg.Data)
			case "History":
				fmt.Println("Your chat history:")
				fmt.Print(msg.Data)
			case "OnlineUsers":
				OnlineUsers = msg.Data.(string)
				fmt.Print("Online users refresh: ", OnlineUsers, "\n")
			case "InviteRequest":
				
			default:
				print("error msg type " + msg.Type)
			}
		}
	}()

	for inp := range input { // send
		send("Chat", inp)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./gochat [ip:port]")
		return
	}
	var wg sync.WaitGroup
	var err error
	f := true

	go signals()
	go ReadInput()

	print(`
    +---------------------------------------+
    |    ____        ____ _           _     |
    |   / ___| ___  / ___| |__   ____| |_   |
    |  | |  _ / _ \| |   | '_ \ / _  | __|  |
    |  | |_| | (_) | |___| | | | (_| | |_   |
    |   \____|\___/ \____|_| |_|\__,_|\__|  |
    |                                       |
    +---------------------------------------+
	
      Welcome to GoChat! Press h for help.
`)

	var addr = "ws://" + os.Args[1] + "/ws"
	conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
	C(err)
	go read(conn)
	go ping(conn)
	for f {
		wg.Add(1)
		f = login(&wg)
	}
	wg.Add(1)
	dialup(&wg)

	wg.Wait()
	exit <- syscall.SIGQUIT
	fmt.Println("Exited.")
}
