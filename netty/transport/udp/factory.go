/*
 *  Copyright 2020 the go-netty project
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       https://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package udp

import (
	"errors"
	"fmt"
	"net"
	"vilan/app"
	"vilan/netty/reuse"
	"vilan/netty/transport"
)

// New udp transport factory
func New() transport.Factory {
	return new(udpFactory)
}

type udpFactory struct{}

func (*udpFactory) Schemes() transport.Schemes {
	return transport.Schemes{"udp", "udp4", "udp6"}
}

func (u *udpFactory) Connect(options *transport.Options) (transport.Transport, error) {

	if err := u.Schemes().FixedURL(options.Address); nil != err {
		return nil, err
	}

	_, err := net.ResolveUDPAddr(options.Address.Scheme, options.Address.Host)
	if nil != err {
		return nil, err
	}
	var d = net.Dialer{
		Control: reuse.Control,
	}
	if options.LocalAddr != nil {
		d.LocalAddr = options.LocalAddr
	}
	conn, err := d.DialContext(options.Context, options.Address.Scheme, options.Address.Host)
	//conn, err := net.DialUDP(options.Address.Scheme, nil, dailAddr)
	if nil != err {
		return nil, err
	}
	udpConn := conn.(*net.UDPConn)
	if udpConn == nil {
		return nil, errors.New("udp connect creat failed")
	}
	return newUDPClientTransport(udpConn), nil
}

func (u *udpFactory) Listen(options *transport.Options) (transport.Acceptor, error) {

	if err := u.Schemes().FixedURL(options.Address); nil != err {
		return nil, err
	}

	_, err := net.ResolveUDPAddr(options.Address.Scheme, options.Address.Host)
	if nil != err {
		return nil, err
	}

	//l, err := net.ListenUDP(options.Address.Scheme, listenAddr)
	l, err := reuse.ListenPacket(options.Address.Scheme, options.AddressWithoutHost())
	if nil != err {
		return nil, err
	}

	udpOptions := FromContext(options.Context, DefaultOptions)

	ua := &udpAcceptor{
		listener:   l.(*net.UDPConn),
		options:    udpOptions,
		transports: make(map[string]*udpServerTransport),
		incoming:   make(chan *udpServerTransport, udpOptions.MaxBacklog),
		closed:     make(chan struct{}),
	}

	go ua.mainLoop()
	return ua, nil
}

type udpAcceptor struct {
	listener   *net.UDPConn
	options    *Options
	transports map[string]*udpServerTransport
	incoming   chan *udpServerTransport
	closed     chan struct{}
}

func (u *udpAcceptor) Accept() (transport.Transport, error) {

	select {
	case <-u.closed:
		return nil, fmt.Errorf("udp listener closed")
	case t := <-u.incoming:
		return t, nil
	}
}

func (u *udpAcceptor) Close() error {

	if nil != u.listener {
		select {
		case <-u.closed:
			return fmt.Errorf("close a closed listener")
		default:
			close(u.closed)
			return u.listener.Close()
		}
	}

	return nil
}

func (u *udpAcceptor) mainLoop() {
	if err := recover(); err != nil {
		app.Logger.Error("UDP mainLoop err:", err)
	}
	defer app.Logger.Error("UDP mainLoop exit ...........................")
	var buffer = make([]byte, u.options.MaxPacketSize)

	for {
		n, raddr, err := u.listener.ReadFromUDP(buffer[:])
		if nil != err {
			// closed all child transports.
			for key, trans := range u.transports {
				delete(u.transports, key)
				_ = trans.Close()
			}
			app.Logger.Error("listener.ReadFromUDP err happened:", err.Error())
			//return
			continue
		}

		trans, ok := u.transports[raddr.String()]
		if !ok {
			trans = newUDPServerTransport(u.listener, raddr)

			select {
			case u.incoming <- trans:
				u.transports[raddr.String()] = trans
			default:
				// acceptor is too slower
				continue
			}
		}

		// copy packet data.
		packet := make([]byte, n)
		copy(packet, buffer[:n])

		// push received packet.
		if !trans.received(packet) {
			// remove the closed transport.
			delete(u.transports, raddr.String())
		}
	}

}
