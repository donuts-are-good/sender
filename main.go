package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Message struct {
	From    string
	Relay   string
	Content []byte
}

type Node struct {
	addr  string
	conns map[string]net.Conn
}

func NewNode(addr string) *Node {
	return &Node{addr, make(map[string]net.Conn)}
}

func (n *Node) Listen() {
	ln, err := net.Listen("tcp", n.addr)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go n.handleConn(conn)
	}
}

func (n *Node) handleConn(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	for {
		var msg Message
		err := dec.Decode(&msg)
		if err != nil {
			return
		}
		fmt.Printf("%s: %s\n", msg.From, msg.Content)
	}
}

func (n *Node) Connect(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	n.conns[addr] = conn
}

func (n *Node) SendMessage(msg Message) {
	var wg sync.WaitGroup
	for _, conn := range n.conns {
		wg.Add(1)
		go func(conn net.Conn) {
			defer wg.Done()
			enc := gob.NewEncoder(conn)
			enc.Encode(msg)
		}(conn)
	}
	wg.Wait()
}

func main() {
	relayPort := flag.String("enable-relay", "", "Enable public relay at the specified port")
	ourPort := flag.String("port", "14000", "Set our listening port (default: 14000)")
	flag.Parse()

	if *relayPort != "" {
		go func() {
			ln, err := net.Listen("tcp", ":"+*relayPort)
			if err != nil {
				panic(err)
			}
			for {
				conn, err := ln.Accept()
				if err != nil {
					continue
				}
				go func(conn net.Conn) {
					defer conn.Close()
					remote, err := net.Dial("tcp", conn.RemoteAddr().String())
					if err != nil {
						return
					}
					defer remote.Close()
					go func() { _, _ = io.Copy(conn, remote) }()
					_, _ = io.Copy(remote, conn)
				}(conn)
			}
		}()
	}

	node := NewNode(":" + *ourPort)
	go node.Listen()

	switch *ourPort {
	case "14000":
		node.Connect("127.0.0.1:14001")
		node.Connect("127.0.0.1:14002")
	case "14001":
		node.Connect("127.0.0.1:14000")
		node.Connect("127.0.0.1:14002")
	case "14002":
		node.Connect("127.0.0.1:14000")
		node.Connect("127.0.0.1:14001")
	default:
		fmt.Println("Invalid port specified.")
		return
	}

	// why isn't this sending messages continuously?
	for {
		msg := Message{*ourPort, "", []byte("Hello :) " + time.Now().String())}
		node.SendMessage(msg)
	}
	// select {}
}
