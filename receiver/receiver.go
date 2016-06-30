package receiver

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// TCP receive metrics from TCP connections
type TCP struct {
	out      chan *string
	err      chan *string
	listener *net.TCPListener
}

// NewTCP create new instance of TCP
func NewTCP(out chan *string) *TCP {
	return &TCP{
		out: out,
	}
}

func (rcv *TCP) handleConnection(conn net.Conn) {

	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
		line, err := reader.ReadBytes('\n')

		if err != nil {
			if err == io.EOF {
				if len(line) > 0 {
					fmt.Printf("[tcp] Unfinished line: %#v", line)
				}
			} else {
				panic(err)
			}
			break
		}
		if len(line) > 0 { // skip empty lines
			var s = string(line)
			rcv.out <- &s
		}
	}
}

// Listen bind port. Receive messages and send to out channel
func (rcv *TCP) Listen(addr *net.TCPAddr) error {

	tcpListener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}

	defer tcpListener.Close()
	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			fmt.Printf("[tcp] Failed to accept connection: %s", err)
			continue
		}
		go rcv.handleConnection(conn)
	}
	return nil
}
