package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func main() {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_TCP)
	if err != nil {
		println("failed to create a socket")
		return
	}
	defer unix.Close(fd)

	addr := &unix.SockaddrInet4{
		Port:8080,
	}
	err = unix.Bind(fd, addr)
	if err != nil {
		log.Println("failed to bind to addr: ", addr.Addr, addr.Port)
		return
	}

	err = unix.Listen(fd, 1)
	if err != nil {
		log.Println("failed to listen to socket : ", addr.Addr, addr.Port)
		return
	}

	serverErr := make(chan os.Signal, 1)
	signal.Notify(serverErr, syscall.SIGINT, syscall.SIGTERM)


	go func() {
		for {
			rqfd, rqsa, err := unix.Accept(fd)
			if err != nil {
				fmt.Println(err)
				panic("accept failed")
			}
			go handler(rqfd, rqsa)
		}
	}()

	<- serverErr
}

func handler(rqfd int, rqsa unix.Sockaddr) {
	defer unix.Close(rqfd)
	// get request message
	requestMsg := make([]byte, 1500)
	var saIPv4 *unix.SockaddrInet4
	switch sa := rqsa.(type) {
	case *unix.SockaddrInet4:
		saIPv4 = &unix.SockaddrInet4{
			Port:sa.Port,
			Addr: sa.Addr,
		}
	default:
		panic("wrong type")
	}

	n, _, err := unix.Recvfrom(rqfd, requestMsg, unix.MSG_WAITALL)
	requestMsg = requestMsg[:n]
	if err != nil && err != io.EOF {
		fmt.Println("failed to get request message")
		panic(err)
	}
	fmt.Println(string(requestMsg))

	// connect to the real server
	connfd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_TCP)
	if err != nil {
		println("failed to create a socket")
		return
	}
	defer unix.Close(connfd)
	serverAddr := &unix.SockaddrInet4{
		Port: 9000,
		Addr: [4]byte{0, 0, 0, 0},
	}
	err = unix.Connect(connfd, serverAddr)
	if err != nil {
		fmt.Println("connect has failed", err)
		panic(err)
	}

	// forward the request to the real server
	err = unix.Sendto(connfd, requestMsg, unix.MSG_SEND, &unix.SockaddrInet4{
		Port: 9000,
		Addr: [4]byte{0, 0, 0, 0},
	})
	if err != nil {
		fmt.Println("failed to forward a message")
		panic(err)
	}

	// get response from the real server
	respMsg := make([]byte, 1500)
	n, _, err = unix.Recvfrom(connfd, respMsg, 0)
	respMsg = respMsg[:n]
	if err != nil {
		panic(err)
	}

	respMsg = bytes.Split(respMsg, []byte("\r\n\r\n"))[1]

	resp := []byte(fmt.Sprintf("HTTP/1.1 200 OK\nContent-Type: text/html\nContent-Length: %v\nAccept-Ranges: bytes\nConnection: close\r\n\r\n", len(respMsg)))
	resp = append(resp, respMsg...)
	resp = append(resp, []byte("\n")...)

	// respond to the client
	err = unix.Sendto(rqfd, resp, unix.MSG_SEND, saIPv4)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(resp))

	fmt.Println("finished")
}

