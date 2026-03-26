package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/organic-programming/go-holons/pkg/transport"
)

func main() {
	lis, err := transport.Listen("tcp://127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer lis.Close()

	fmt.Printf("tcp://%s\n", lis.Addr().String())
	if tcpLis, ok := lis.(*net.TCPListener); ok {
		_ = tcpLis.SetDeadline(time.Now().Add(5 * time.Second))
	}

	conn, err := lis.Accept()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()

	if _, err := io.Copy(conn, conn); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
