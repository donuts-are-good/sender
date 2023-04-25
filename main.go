package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Node struct {
	ID        ed25519.PublicKey
	Signature []byte
	Addr      *net.TCPAddr
}

type Message struct {
	Topic     string
	From      *Node
	Relay     *Node
	Content   []byte
	Signature []byte
}

type Network struct {
	addr  string
	conns map[string]net.Conn
	node  *Node
}

func main() {
	relayPort := flag.String("enable-relay", "", "Enable public relay at the specified port")
	ourPort := flag.String("port", "14000", "Set our listening port (default: 14000)")
	local := flag.Bool("local", false, "Use 127.0.0.1 for testing purposes")
	flag.Parse()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	_, externalIP, err := GetIPs()
	if err != nil {
		panic(err)
	}

	if *local {
		externalIP = net.ParseIP("127.0.0.1")
	}

	node := &Node{
		ID:   publicKey,
		Addr: &net.TCPAddr{IP: externalIP, Port: 14000},
	}
	node.Signature = ed25519.Sign(privateKey, publicKey)

	network := NewNetwork(":"+*ourPort, node)

	if *relayPort != "" {
		relayPortInt, err := strconv.Atoi(*relayPort)
		if err != nil {
			panic(err)
		}
		relayNode := &Node{
			ID:   publicKey,
			Addr: &net.TCPAddr{IP: externalIP, Port: relayPortInt},
		}
		relayNode.Signature = ed25519.Sign(privateKey, publicKey)

		relayNetwork := NewNetwork(":"+*relayPort, relayNode)
		go relayNetwork.Listen()
	}

	go network.Listen()

	// Connect to other nodes and send messages
	ourPortInt, err := strconv.Atoi(*ourPort)
	if err != nil {
		panic(err)
	}

	delayBetweenMessages := time.Second * 5 // Adjust the delay as needed

	for i := 14000; i <= 14002; i++ {
		if i != ourPortInt {
			peerAddr := fmt.Sprintf("127.0.0.1:%d", i)
			network.Connect(peerAddr)
			go sendMessageContinuously(network, peerAddr, delayBetweenMessages, privateKey)
		}
	}

	select {}
}

func NewNetwork(addr string, node *Node) *Network {
	return &Network{addr, make(map[string]net.Conn), node}
}

func (n *Network) Listen() {
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

func (n *Network) handleConn(conn net.Conn) {
	defer conn.Close()
	dec := gob.NewDecoder(conn)

	for {
		var msg Message
		err := dec.Decode(&msg)
		if err == io.EOF {
			// Connection closed, exit the loop
			break
		} else if err != nil {
			fmt.Printf("Error decoding message: %v\n", err)
			continue
		}

		if !ed25519.Verify(msg.From.ID, msg.Content, msg.Signature) {
			fmt.Println("Invalid message signature!")
			continue
		}

		fmt.Printf("%s: %s\n", msg.From.ID, msg.Content)
	}
}

func sendMessageContinuously(network *Network, peerAddr string, delay time.Duration, privkey ed25519.PrivateKey) {
	for {
		network.SendMessage("general", []byte(fmt.Sprintf("Hello, %s!", peerAddr)), privkey)
		time.Sleep(delay)
	}
}
func (n *Network) Connect(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	n.conns[addr] = conn
}

func (n *Network) SendMessage(topic string, content []byte, privkey ed25519.PrivateKey) {
	msg := Message{
		Topic:   topic,
		From:    n.node,
		Content: content,
	}
	msg.Signature = ed25519.Sign(privkey, content)

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

func GetIPs() (net.IP, net.IP, error) {
	conn, err := net.Dial("tcp", "icanhazip.com:80")
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()

	internalIP := conn.LocalAddr().(*net.TCPAddr).IP

	// Send request to icanhazip.com
	fmt.Fprintf(conn, "GET / HTTP/1.0\r\nHost: icanhazip.com\r\n\r\n")
	resp, err := io.ReadAll(conn)
	if err != nil {
		return nil, nil, err
	}

	externalIP := net.ParseIP(strings.TrimSpace(string(resp)))
	return internalIP, externalIP, nil
}
