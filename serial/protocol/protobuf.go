package message

import (
	"encoding/binary"
	"fmt"
	"github.com/golang/protobuf/proto"
	"sync"
	"vilan/netty"
	"vilan/netty/codec"
	"vilan/netty/utils"
)

const PackLength = 2048

// scheme :0 -> tcp; !=0 -> udp
func ProtobufCodec(scheme uint32, maxFrameLength uint32) codec.Codec {
	utils.AssertIf(maxFrameLength <= 0, "maxFrameLength must be a positive integer")
	return &protobufCodec{
		scheme:         scheme,
		maxFrameLength: maxFrameLength,
		frameLength:    0,
		buffers:        sync.Pool{New: func() interface{} { return new([PackLength]byte) }},
	}
}

type protobufCodec struct {
	scheme         uint32
	buffer         []byte
	frameLength    uint32
	maxFrameLength uint32
	buffers        sync.Pool
}

func (v *protobufCodec) CodecName() string {
	return "protobuf-codec"
}

func (v *protobufCodec) HandleRead(ctx netty.InboundContext, message netty.Message) {
	reader := utils.MustToReader(message)
	data := v.buffers.Get().(*[PackLength]byte) //make([]byte, v.maxFrameLength)
	rn, err := reader.Read(data[:])
	if rn <= 0 && err != nil {
		return
	}
	// udp
	if v.scheme != 0 {
		frameLength, num := utils.Uvarint32(data[:])
		if num == 0 && rn < 5 || frameLength > v.maxFrameLength { // 不完整帧头，或过大
			return
		}
		if num >= rn-2 {
			return
		}
		if data[num] == 0xF8 && data[num+1] == 0x01 {
			msg := &MsgServerFrame{}
			if err := proto.Unmarshal(data[num:rn], msg); err == nil {
				ctx.HandleRead(msg)
			}
		} else {
			msg := &MsgClientFrame{}
			if err := proto.Unmarshal(data[num:rn], msg); err == nil {
				ctx.HandleRead(msg)
			}
		}
	} else { // tcp
		if v.buffer == nil || len(v.buffer) == 0 {
			v.buffer = data[:rn]
		} else {
			v.buffer = append(v.buffer, data[:rn]...)
		}
		v.readAndDeal(ctx)
	}

}

func (v *protobufCodec) readAndDeal(ctx netty.InboundContext) {
	defer func() {
		if err := recover(); err != nil {
			v.frameLength = 0
			v.buffer = nil
		}
	}()
	var num int
	var frameLength uint32
	if v.frameLength == 0 {
		frameLength, num = utils.Uvarint32(v.buffer)
		if num == 0 && len(v.buffer) < 5 || frameLength > 2*1024*1024 { // 不完整帧头，最大2M
			v.frameLength = 0
			return
		}
		utils.AssertIf(num < 0, "n < 0: value larger than 64 bits")
		v.frameLength = frameLength
		v.buffer = v.buffer[num:]
	} else {
		frameLength = v.frameLength
		num = 0
	}

	if len(v.buffer) == int(frameLength) {
		data := v.buffer
		v.buffer = nil
		v.frameLength = 0
		if data[0] == 0xF8 && data[1] == 0x01 {
			msg := &MsgServerFrame{}
			if err := proto.Unmarshal(data, msg); err == nil {
				ctx.HandleRead(msg)
			}
		} else {
			msg := &MsgClientFrame{}
			if err := proto.Unmarshal(data, msg); err == nil {
				ctx.HandleRead(msg)
			}
		}
		return
	} else if len(v.buffer) > int(frameLength) {
		for len(v.buffer) > int(frameLength) {
			data := v.buffer[:frameLength]
			v.buffer = v.buffer[frameLength:]
			if data[0] == 0xF8 && data[1] == 0x01 {
				msg := &MsgServerFrame{}
				if err := proto.Unmarshal(data, msg); err == nil {
					ctx.HandleRead(msg)
				}
			} else {
				msg := &MsgClientFrame{}
				if err := proto.Unmarshal(data, msg); err == nil {
					ctx.HandleRead(msg)
				}
			}
			frameLength, num = utils.Uvarint32(v.buffer)
			if num == 0 && len(v.buffer) < 5 || frameLength > 2*1024*1024 { // 不完整帧头，最大2M
				v.frameLength = 0
				return
			}
			utils.AssertIf(num < 0, "n < 0: value larger than 64 bits")
			v.frameLength = frameLength
			v.buffer = v.buffer[num:]
		}
		if len(v.buffer) == int(frameLength) {
			data := v.buffer
			v.buffer = nil
			v.frameLength = 0
			if data[0] == 0xF8 && data[1] == 0x01 { // field index
				msg := &MsgServerFrame{}
				if err := proto.Unmarshal(data, msg); err == nil {
					ctx.HandleRead(msg)
				}
			} else {
				msg := &MsgClientFrame{}
				if err := proto.Unmarshal(data, msg); err == nil {
					ctx.HandleRead(msg)
				}
			}
			return
		} else if len(v.buffer) < int(frameLength) {
			return
		}
	} else {
	}
}

func (v *protobufCodec) HandleWrite(ctx netty.OutboundContext, message netty.Message) {
	if message == nil {
		return
	}
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(fmt.Sprintf("msg encode failed:%s", err))
		}
	}()

	switch v := message.(type) {
	case *MsgClientFrame:
		data, err := proto.Marshal(v)
		if err != nil {
			return
		}
		// encode header
		var head = [binary.MaxVarintLen32]byte{}
		n := utils.PutUvarint32(head[:], uint32(len(data)))

		// optimize one merge operation to reduce memory allocation.
		ctx.HandleWrite([][]byte{
			// header
			head[:n],
			// payload
			data,
		})
	case *MsgServerFrame:
		data, err := proto.Marshal(v)
		if err != nil {
			return
		}
		// encode header
		var head = [binary.MaxVarintLen32]byte{}
		n := utils.PutUvarint32(head[:], uint32(len(data)))

		// optimize one merge operation to reduce memory allocation.
		ctx.HandleWrite([][]byte{
			// header
			head[:n],
			// payload
			data,
		})
	default:
		bodyBytes := utils.MustToBytes(message)
		// encode header
		var head = [binary.MaxVarintLen32]byte{}
		n := utils.PutUvarint32(head[:], uint32(len(bodyBytes)))

		// optimize one merge operation to reduce memory allocation.
		ctx.HandleWrite([][]byte{
			// header
			head[:n],
			// payload
			bodyBytes,
		})
		return
	}
}
