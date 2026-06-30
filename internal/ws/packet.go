package ws

import (
	"encoding/binary"
	"fmt"
)

const (
	headerLen   = 10
	packHeadLen = 4
	typeLenSize = 2
	roomIdSize  = 4
)

type Packet struct {
	PackType string
	RoomID   uint32
	Body     []byte
}

func Pack(pt string, roomID uint32, body []byte) []byte {
	typeLen := len(pt)
	packLen := typeLenSize + typeLen + roomIdSize + len(body)
	buf := make([]byte, packHeadLen+packLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(packLen))
	binary.BigEndian.PutUint16(buf[packHeadLen:], uint16(typeLen))
	copy(buf[packHeadLen+typeLenSize:], pt)
	binary.BigEndian.PutUint32(buf[packHeadLen+typeLenSize+typeLen:], roomID)
	copy(buf[packHeadLen+typeLenSize+typeLen+roomIdSize:], body)
	return buf
}

func Unpack(data []byte) (*Packet, []byte, error) {
	if len(data) < packHeadLen {
		return nil, data, nil
	}
	packLen := binary.BigEndian.Uint32(data[0:4])
	if len(data) < int(packHeadLen+packLen) {
		return nil, data, nil
	}
	payload := data[packHeadLen : packHeadLen+packLen]
	rest := data[packHeadLen+packLen:]

	if len(payload) < typeLenSize {
		return nil, rest, fmt.Errorf("payload too short for type length")
	}
	typeLen := int(binary.BigEndian.Uint16(payload[0:2]))
	if len(payload) < typeLenSize+typeLen+roomIdSize {
		return nil, rest, fmt.Errorf("payload too short for type+roomID")
	}
	packType := string(payload[typeLenSize : typeLenSize+typeLen])
	roomID := binary.BigEndian.Uint32(payload[typeLenSize+typeLen : typeLenSize+typeLen+roomIdSize])
	body := payload[typeLenSize+typeLen+roomIdSize:]

	return &Packet{PackType: packType, RoomID: roomID, Body: body}, rest, nil
}
