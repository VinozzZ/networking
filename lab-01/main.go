package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sort"
)

type Header interface {
	parseHeader(buffer *bytes.Buffer)
}

type GlobalHeader struct {
	MagicNumber         uint32
	MajorVersion        uint16
	MinorVersion        uint16
	TimeZoneOffSet      uint32
	TimestampAccuracy   uint32
	SnapshotLength      uint32
	LinkLayerHeaderType uint32
}

type PacketHeader struct {
	TimestampSeconds  uint32
	TimestampNanos    uint32
	DataLength        uint32
	UntruncatedLength uint32
}

// Ethernet is the layer for Ethernet frame headers.
type Ethernet struct {
	SrcMAC, DstMAC [6]byte
	EthernetType   uint16
}

// IP is the layer for IP datagram header.
type IP struct {
	VersionAndIHL        uint8
	_                    uint8
	TotalLength          uint16
	_                    uint32
	_                    uint8
	Protocol             uint8
	Checksum             uint16
	SourceIPAddress      [4]byte
	DestinationIPAddress [4]byte
}

type TCP struct {
	SourcePort           uint16
	DestinationPort      uint16
	SequenceNumber       uint32
	AcknowledgmentNumber uint32
	DataOffsetAndFlags  uint8
	Flags                uint8
	WindowSize           uint16
	Checksum             uint16
	UrgentPointer        uint16
}

func main() {
	// open the file
	file, err := os.Open("net.cap")
	if err != nil {
		panic("failed to open the file")
	}
	defer file.Close()

	// get the file size
	stat, err := file.Stat()
	if err != nil {
		panic("failed to get file size")
	}

	// read the file into a buffer
	data := make([]byte, stat.Size())
	_, err = file.Read(data)
	if err != nil && err != io.EOF {
		panic("failed to read the file into a buffer")
	}
	buffer := bytes.NewBuffer(data)

	// get global header
	globalHeader := &GlobalHeader{}
	if err := binary.Read(buffer, binary.LittleEndian, globalHeader); err != nil {
		log.Panicf("failed to get global header: %s", err)
	}

	if globalHeader.MagicNumber == 0xa1b2c3d4 {
		fmt.Println("Little Endian")
	}
	fmt.Printf("-----------Global Header--------- \n %+v \n", globalHeader)

	httpData := make(map[uint32][]byte)
	for err != io.EOF {
		// get packet header
		packetHeader := &PacketHeader{}
		if err = binary.Read(buffer, binary.LittleEndian, packetHeader); err != nil {
			if err == io.EOF {
				break
			}
			log.Panicf("failed to get packet header: %s", err)
		}

		//get packet data
		packetData := make([]byte, packetHeader.DataLength)
		if err = binary.Read(buffer, binary.BigEndian, packetData); err != nil {
			log.Panicf("failed to get packet data: %s", err)
		}

		packetBuffer := bytes.NewBuffer(packetData)

		ethernetHeader := &Ethernet{}
		parseLayers(packetBuffer, ethernetHeader)


		ipHeader := &IP{}
		parseLayers(packetBuffer, ipHeader)
		ipHeaderLength := ipHeader.VersionAndIHL & 0x0f * 4

		tcpHeader := &TCP{}
		parseLayers(packetBuffer, tcpHeader)
		tcpHeaderLength := tcpHeader.DataOffsetAndFlags >> 4 * 4

		// Discard padding in the header
		if tcpHeaderLength > 20 {
			io.CopyN(ioutil.Discard, packetBuffer, int64(tcpHeaderLength-20))
		}

		// check SYN message
		if (tcpHeader.Flags >> 1) & 1 == 1 {
			continue
		}

		dataSize := ipHeader.TotalLength - uint16(ipHeaderLength) - uint16(tcpHeaderLength)
		tcpData := make([]byte, dataSize)

		n, err := io.ReadFull(packetBuffer, tcpData)
		if err != nil || uint16(n) != dataSize {
			fmt.Println("something went wrong", err, dataSize, n)
			continue
		}
		if _, ok := httpData[tcpHeader.SequenceNumber]; ok {
			continue
		}
		httpData[tcpHeader.SequenceNumber] = tcpData

		// check FIN message
		if tcpHeader.Flags & 1 == 1 {
			break
		}
	}

	nums := make([]int, 0)
	for sequence, _ := range httpData {
		nums = append(nums, int(sequence))
	}
	sort.Ints(nums)

	f, _ := os.Create("./img.jpg")
	defer f.Close()


	httpHeaderIdx := bytes.Index(httpData[uint32(nums[0])], []byte("\r\n\r\n"))
	if httpHeaderIdx != -1 {
		httpData[uint32(nums[0])] = httpData[uint32(nums[0])][httpHeaderIdx+4:]
	}

	for _, sequence := range nums {
		_, err = f.Write(httpData[uint32(sequence)])
	}
}

func (e *Ethernet) parseHeader(buffer *bytes.Buffer) {
		// parse link layer
	if err := binary.Read(buffer, binary.BigEndian, e); err != nil {
		log.Panic("failed to parse link layer header", err)
	}
}

func (i *IP) parseHeader(buffer *bytes.Buffer) {
		// parse network layer
	if err := binary.Read(buffer, binary.BigEndian, i); err != nil {
		log.Panic("failed to parse network layer header", err)
	}
}

func (t *TCP) parseHeader(buffer *bytes.Buffer) {
		// parse transport layer
	if err := binary.Read(buffer, binary.BigEndian, t); err != nil {
		log.Panic("failed to parse transport layer header", err)
	}
}

func parseLayers(buffer *bytes.Buffer, header Header) {
	header.parseHeader(buffer)
}
