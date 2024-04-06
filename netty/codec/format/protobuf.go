/*
 * Copyright 2019 the go-netty project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package format

import (
	"encoding/binary"
	"fmt"
	"github.com/golang/protobuf/proto"
	"vilan/app"
	"vilan/netty"
	"vilan/netty/codec"
	"vilan/netty/utils"
	"vilan/protocol"
)

const PackLength = 2048
const ExSize = 30

// scheme :0 -> tcp; !=0 -> udp
func ProtobufCodec(scheme uint32, maxFrameLength uint32) codec.Codec {
	utils.AssertIf(maxFrameLength <= 0, "maxFrameLength must be a positive integer")
	return &protobufCodec{
		scheme:         scheme,
		maxFrameLength: maxFrameLength,
		frameLength:    0,
		packetBuffer:   []byte{},
		rxBuffer:       make([]byte, PackLength),
		txBuffer:       make([]byte, PackLength),
	}
}

type protobufCodec struct {
	scheme         uint32
	packetBuffer   []byte
	rxBuffer       []byte
	txBuffer       []byte
	frameLength    uint32
	maxFrameLength uint32
}

func (v *protobufCodec) CodecName() string {
	return "protobuf-codec"
}
func (v *protobufCodec) HandleRead(ctx netty.InboundContext, message netty.Message) {
	reader := utils.MustToReader(message)

	rn, err := reader.Read(v.rxBuffer[:])
	if rn <= 0 && err != nil {
		return
	}
	// udp
	if v.scheme != 0 {
		frameLength, num := utils.Uvarint32(v.rxBuffer[:])
		if num == 0 && rn < 5 || frameLength > v.maxFrameLength { // 不完整帧头，或过大
			return
		}
		if num >= rn-2 {
			return
		}
		if v.rxBuffer[num] == 144 && v.rxBuffer[num+1] == 3 {
			msg := &protocol.MsgDataFrame{}
			if err := proto.Unmarshal(v.rxBuffer[num:rn], msg); err == nil {
				app.RuntimeService.SetStats(uint64(ExSize+len(msg.Data)), true, msg.Token == 0)
				if len(msg.Data) > 0 {
					if l, e := app.RuntimeService.DecryptMsg(msg, v.txBuffer[:]); e == nil {
						msg.Data = v.txBuffer[:l]
					}
					ctx.HandleRead(msg)
				} else {
					ctx.HandleRead(msg)
				}
			}
		} else {
			msg := &protocol.MsgServerFrame{}
			if err := proto.Unmarshal(v.rxBuffer[num:rn], msg); err == nil {
				app.RuntimeService.SetStats(uint64(msg.XXX_Size()), true, false)
				ctx.HandleRead(msg)
			}
		}
	} else { // tcp
		if v.packetBuffer == nil {
			v.packetBuffer = []byte{}
		}
		v.packetBuffer = append(v.packetBuffer, v.rxBuffer[:rn]...)
		v.readAndDeal(ctx)
	}

}

func (v *protobufCodec) readAndDeal(ctx netty.InboundContext) {
	defer func() {
		if err := recover(); err != nil {
			v.frameLength = 0
			v.packetBuffer = []byte{}
		}
	}()
	var num int
	var frameLength uint32
	if v.frameLength == 0 {
		frameLength, num = utils.Uvarint32(v.packetBuffer[:])
		if num == 0 && len(v.packetBuffer) < 5 || frameLength > 2*1024*1024 { // 不完整帧头，或过大
			v.frameLength = 0
			return
		}
		utils.AssertIf(num < 0, "n < 0: value larger than 64 bits")
		v.frameLength = frameLength
		v.packetBuffer = v.packetBuffer[num:]
	} else {
		frameLength = v.frameLength
		num = 0
	}

	if len(v.packetBuffer) == int(frameLength) {
		data := v.packetBuffer
		v.packetBuffer = nil
		v.frameLength = 0
		if data[0] == 144 && data[1] == 3 { // field index 区分
			msg := &protocol.MsgDataFrame{}
			if err := proto.Unmarshal(data, msg); err == nil {
				app.RuntimeService.SetStats(uint64(ExSize+len(msg.Data)), true, msg.Token == 0)
				if len(msg.Data) > 0 {
					if l, e := app.RuntimeService.DecryptMsg(msg, v.txBuffer[:]); e == nil {
						msg.Data = v.txBuffer[:l]
					}
					ctx.HandleRead(msg)
				} else {
					ctx.HandleRead(msg)
				}
			}
		} else {
			msg := &protocol.MsgServerFrame{}
			if err := proto.Unmarshal(data, msg); err == nil {
				app.RuntimeService.SetStats(uint64(msg.XXX_Size()), true, false)
				ctx.HandleRead(msg)
			}
		}
		return
	} else if len(v.packetBuffer) > int(frameLength) {
		for len(v.packetBuffer) > int(frameLength) {
			data := v.packetBuffer[:frameLength]
			v.packetBuffer = v.packetBuffer[frameLength:]
			if data[0] == 144 && data[1] == 3 {
				msg := &protocol.MsgDataFrame{}
				if err := proto.Unmarshal(data, msg); err == nil {
					app.RuntimeService.SetStats(uint64(ExSize+len(msg.Data)), true, msg.Token == 0)
					if len(msg.Data) > 0 {
						if l, e := app.RuntimeService.DecryptMsg(msg, v.txBuffer[:]); e == nil {
							msg.Data = v.txBuffer[:l]
						}
						ctx.HandleRead(msg)
					} else {
						ctx.HandleRead(msg)
					}
				}
			} else {
				msg := &protocol.MsgServerFrame{}
				if err := proto.Unmarshal(data, msg); err == nil {
					app.RuntimeService.SetStats(uint64(msg.XXX_Size()), true, false)
					ctx.HandleRead(msg)
				}
			}
			frameLength, num = utils.Uvarint32(v.packetBuffer[:])
			if num == 0 && len(v.packetBuffer) < 5 || frameLength > 2*1024*1024 { // 不完整帧头，最大2M
				v.frameLength = 0
				return
			}
			utils.AssertIf(num < 0, "n < 0: value larger than 64 bits")
			v.frameLength = frameLength
			v.packetBuffer = v.packetBuffer[num:]
		}
		if len(v.packetBuffer) == int(frameLength) {
			data := v.packetBuffer
			v.packetBuffer = []byte{}
			v.frameLength = 0
			if data[0] == 144 && data[1] == 3 {
				msg := &protocol.MsgDataFrame{}
				if err := proto.Unmarshal(data, msg); err == nil {
					app.RuntimeService.SetStats(uint64(ExSize+len(msg.Data)), true, msg.Token == 0)
					if len(msg.Data) > 0 {
						if l, e := app.RuntimeService.DecryptMsg(msg, v.txBuffer[:]); e == nil {
							msg.Data = v.txBuffer[:l]
						}
						ctx.HandleRead(msg)
					} else {
						ctx.HandleRead(msg)
					}
				}
			} else {
				msg := &protocol.MsgServerFrame{}
				if err := proto.Unmarshal(data, msg); err == nil {
					app.RuntimeService.SetStats(uint64(msg.XXX_Size()), true, false)
					ctx.HandleRead(msg)
				}
			}
			return
		} else if len(v.packetBuffer) < int(frameLength) {
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
	switch m := message.(type) {
	case *protocol.MsgDataFrame:
		switch m.MsgType {
		case protocol.MsgType_Msg_Packet:
			if len(m.Data) > 0 {
				if l, e := app.RuntimeService.EncryptMsg(m, v.txBuffer[:]); e == nil {
					m.Data = v.txBuffer[:l]
				}
			}
			app.RuntimeService.SetStats(uint64(ExSize+len(m.Data)), false, m.Token == 0)
			break
		default:
			app.RuntimeService.SetStats(uint64(m.XXX_Size()), false, m.Token == 0)
		}
		data, err := proto.Marshal(m)
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
	case *protocol.MsgPeerFrame:
		app.RuntimeService.SetStats(uint64(m.XXX_Size()), false, m.Token == 0)
		data, err := proto.Marshal(m)
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
