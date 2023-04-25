package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"sync"
)

type Message struct {
	From    string
	Content string
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
	node := NewNode(":9000")
	go node.Listen()

	node.Connect("10.0.0.2:9000")
	node.Connect("10.0.0.3:9000")

	msg := Message{"Node1", "Hello, World!"}
	node.SendMessage(msg)
}
