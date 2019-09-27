package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"strings"

	"golang.org/x/sys/unix"
)

type Message struct {
	Header     Header
	Question   Question
	Answer     RR
	Authority  RR
	Additional RR
}

type Header struct {
	ID      uint16
	QR      bool
	OPCODE  uint8
	AA      bool
	TC      bool
	RD      bool
	RA      bool
	Z       uint8
	RCODE   uint8
	QDCOUNT uint16
	ANCOUNT uint16
	NSCOUNT uint16
	ARCOUNT uint16
}

type Question struct {
	QNAME  string
	QTYPE  uint16
	QCLASS uint16
}

type RR struct {
	NAME     []byte
	TYPE     uint16
	CLASS    uint16
	TTL      uint32
	RDLENGTH uint16
	RDATA    []byte
}

func main() {
	// open a socket
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		fmt.Println("failed opening an socket", err)
		return
	}
	defer unix.Close(fd)

	// bind to a port
	ipInByte := [4]byte{192, 168, 1, 5}
	socketAddr := &unix.SockaddrInet4{
		Port: 8080,
		Addr: ipInByte,
	}
	err = unix.Bind(fd, socketAddr)
	if err != nil {
		fmt.Println("failed to bind a socket to a port", err)
		return
	}

	// construct the query
	dnsQuery := Message{}
	dnsQuery.Header = Header{
		ID:      0x1111,
		QR:      false,
		OPCODE:  0 << 4,
		QDCOUNT: 1,
	}
	dnsQuery.Question = Question{
		QNAME:  "google.com",
		QTYPE:  1,
		QCLASS: 1,
	}

	reqBuffer := new(bytes.Buffer)
	flags1 := byte(toInt(dnsQuery.Header.QR)<<7 | toInt(false)<<3 | toInt(false)<<1 | toInt(true))
	flags2 := byte(0<<7 | 1<<5)
	binary.Write(reqBuffer, binary.BigEndian, dnsQuery.Header.ID)
	binary.Write(reqBuffer, binary.BigEndian, flags1)
	binary.Write(reqBuffer, binary.BigEndian, flags2)
	binary.Write(reqBuffer, binary.BigEndian, dnsQuery.Header.QDCOUNT)
	binary.Write(reqBuffer, binary.BigEndian, dnsQuery.Header.ANCOUNT)
	binary.Write(reqBuffer, binary.BigEndian, dnsQuery.Header.NSCOUNT)
	binary.Write(reqBuffer, binary.BigEndian, dnsQuery.Header.ARCOUNT)
	domain := packDomain(dnsQuery.Question.QNAME)
	binary.Write(reqBuffer, binary.BigEndian, domain)
	binary.Write(reqBuffer, binary.BigEndian, dnsQuery.Question.QTYPE)
	binary.Write(reqBuffer, binary.BigEndian, dnsQuery.Question.QCLASS)

	queryLength := len(reqBuffer.Bytes())
	respData := make([]byte, queryLength)

	err = unix.Sendto(fd, reqBuffer.Bytes(), unix.MSG_SEND, &unix.SockaddrInet4{
		Port: 53,
		Addr: [4]byte{8, 8, 8, 8},
	})
	if err != nil {
		fmt.Println("failed to send a DNS query", err)
		return
	}

	for {
		n, _, err := unix.Recvfrom(fd, respData, unix.MSG_WAITALL)
		if n == -1 {
			fmt.Println("failed request")
		}
		if err != nil {
			break
		}
		if n == queryLength {
			break
		}
	}
	respBuffer := bytes.NewBuffer(respData)
	respHeader := &Header{}

	binary.Read(respBuffer, binary.BigEndian, respHeader)
	respHeader.OPCODE = respHeader.OPCODE << 4
	fmt.Printf("%+v", respHeader)

}

func toInt(flag bool) int {
	if flag {
		return 1
	}
	return 0
}

func packDomain(domain string) []byte {
	var buffer bytes.Buffer

	domainParts := strings.Split(domain, ".")
	for _, part := range domainParts {
		if err := binary.Write(&buffer, binary.BigEndian, byte(len(part))); err != nil {
			log.Fatalf("Error binary.Write(..) for '%s': '%s'", part, err)
		}

		for _, c := range part {
			if err := binary.Write(&buffer, binary.BigEndian, uint8(c)); err != nil {
				log.Fatalf("Error binary.Write(..) for '%s'; '%c': '%s'", part, c, err)
			}
		}
	}

	binary.Write(&buffer, binary.BigEndian, uint8(0))

	return buffer.Bytes()
}
