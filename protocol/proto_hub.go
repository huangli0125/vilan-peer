package protocol

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"vilan/netty"
)

func WriteP2pMsg(ctx netty.Channel, msg *MsgDataFrame) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = ctx.Write(data)
	return err
}
func WritePeerMsg(ctx netty.Channel, msg *MsgPeerFrame) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = ctx.Write(data)
	return err
}

func ReadP2pMsg(message netty.Message) (msg *MsgDataFrame, err error) {
	if wv, ok := message.([]byte); ok {
		msg = &MsgDataFrame{}
		err = proto.Unmarshal(wv, msg)
		return msg, err
	} else {
		return nil, errors.New("error msg data")
	}
}
func ReadServerMsg(message netty.Message) (msg *MsgServerFrame, err error) {
	if wv, ok := message.([]byte); ok {
		msg = &MsgServerFrame{}
		err = proto.Unmarshal(wv, msg)
		return msg, err
	} else {
		return nil, errors.New("error msg data")
	}
}
