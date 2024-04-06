/*
Goserial is a simple go package to allow you to read and write from
the serial port as a stream of bytes.

It aims to have the same API on all platforms, including windows.  As
an added bonus, the windows package does not use cgo, so you can cross
compile for windows from another platform.  Unfortunately goinstall
does not currently let you cross compile so you will have to do it
manually:

	GOOS=windows make clean install

Currently there is very little in the way of configurability.  You can
set the baud rate.  Then you can Read(), Write(), or Close() the
connection.  Read() will block until at least one byte is returned.
Write is the same.  There is currently no exposed way to set the
timeouts, though patches are welcome.

Currently all ports are opened with 8 data bits, 1 stop bit, no
parity, no hardware flow control, and no software flow control.  This
works fine for many real devices and many faux serial devices
including usb-to-serial converters and bluetooth serial ports.

You may Read() and Write() simulantiously on the same connection (from
different goroutines).

Example usage:

	package main

	import (
	      "github.com/tarm/serial"
	      "log"
	)

	func main() {
	      c := &serial.PlayConfig{Name: "COM5", Baud: 115200}
	      s, err := serial.OpenPort(c)
	      if err != nil {
	              log.Fatal(err)
	      }

	      n, err := s.Write([]byte("test"))
	      if err != nil {
	              log.Fatal(err)
	      }

	      buf := make([]byte, 128)
	      n, err = s.Read(buf)
	      if err != nil {
	              log.Fatal(err)
	      }
	      log.Print("%q", buf[:n])
	}
*/
package serial

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const DefaultSize = 8 // Default value for PlayConfig.Size

type StopBits byte
type Parity byte

const (
	Stop1     StopBits = 1
	Stop1Half StopBits = 15
	Stop2     StopBits = 2
)

const (
	ParityNone  Parity = 78 // 'N'
	ParityOdd   Parity = 79 // 'O'
	ParityEven  Parity = 69 // 'E'
	ParityMark  Parity = 77 // 'M' // parity bit is always 1
	ParitySpace Parity = 83 // 'S' // parity bit is always 0
)

// PlayConfig contains the information needed to open a serial port.
//
// Currently few options are implemented, but more may be added in the
// future (patches welcome), so it is recommended that you create a
// new config addressing the fields by name rather than by order.
//
// For example:
//
//	c0 := &serial.PlayConfig{Name: "COM45", Baud: 115200, ReadTimeout: time.Millisecond * 500}
//
// or
//
//	c1 := new(serial.PlayConfig)
//	c1.Name = "/dev/tty.usbserial"
//	c1.Baud = 115200
//	c1.ReadTimeout = time.Millisecond * 500
type Config struct {
	Name string `json:"name"`
	Baud int    `json:"baud"`
	Size byte   `json:"size"`

	// Parity is the bit to use and defaults to ParityNone (no parity bit).
	Parity Parity `json:"parity"`

	// Number of stop bits to use. Default is 1 (1 stop bit).
	StopBits StopBits `json:"stop_bits"`

	// RTSFlowControl bool
	// DTRFlowControl bool
	// XONFlowControl bool

	// CRLFTranslate bool
	ReadTimeout uint32 `json:"-"` // ms

	RemoteName   string `json:"remote_name"`    // 远端串口名称
	UserPortName string `json:"user_port_name"` // 本地虚拟串口,供用户使用
}

// ErrBadSize is returned if Size is not supported.
var ErrBadSize error = errors.New("unsupported serial data size")

// ErrBadStopBits is returned if the specified StopBits setting not supported.
var ErrBadStopBits error = errors.New("unsupported stop bit setting")

// ErrBadParity is returned if the parity is not supported.
var ErrBadParity error = errors.New("unsupported parity setting")

// OpenPort opens a serial port with the specified configuration
func OpenPort(c *Config) (*Port, error) {
	size, par, stop := c.Size, c.Parity, c.StopBits
	if size == 0 {
		size = DefaultSize
	}
	if par == 0 {
		par = ParityNone
	}
	if stop == 0 {
		stop = Stop1
	}
	return openPort(c.Name, c.Baud, size, par, stop, c.ReadTimeout)
}

// Converts the timeout values for Linux / POSIX systems
func posixTimeoutValues(readTimeout uint32) (vmin uint8, vtime uint8) {
	const MAXUINT8 = 1<<8 - 1 // 255
	// set blocking / non-blocking read
	var minBytesToRead uint8 = 1
	var readTimeoutInDeci int64
	timeout := time.Duration(readTimeout) * time.Millisecond
	if timeout > 0 {
		// EOF on zero read
		minBytesToRead = 0
		// convert timeout to deciseconds as expected by VTIME
		readTimeoutInDeci = timeout.Nanoseconds() / 1e6 / 100
		// capping the timeout
		if readTimeoutInDeci < 1 {
			// min possible timeout 1 Deciseconds (0.1s)
			readTimeoutInDeci = 1
		} else if readTimeoutInDeci > MAXUINT8 {
			// max possible timeout is 255 deciseconds (25.5s)
			readTimeoutInDeci = MAXUINT8
		}
	}
	return minBytesToRead, uint8(readTimeoutInDeci)
}

// func SendBreak()

// func RegisterBreakHandler(func())
var mutex sync.Mutex

func LoadConfig() (map[string]*Config, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if dir, err := filepath.Abs(filepath.Dir(os.Args[0])); err != nil {
		return nil, err
	} else {
		file := dir + "\\config\\serial.json"
		if runtime.GOOS != "windows" {
			file = dir + "/config/serial.json"
		}
		if content, err := ioutil.ReadFile(file); err != nil {
			return nil, err
		} else {
			configs := make(map[string]*Config)
			if err = json.Unmarshal(content, &configs); err != nil {
				return nil, err
			}
			return configs, err
		}
	}
}

func SaveConfig(conf *Config) error {
	if conf == nil || len(conf.Name) == 0 {
		return errors.New("invalid serial config")
	}
	configs, _ := LoadConfig()
	mutex.Lock()
	defer mutex.Unlock()
	if configs == nil {
		configs = make(map[string]*Config)
	}
	conf.RemoteName = ""
	configs[conf.Name] = conf
	if content, err := json.Marshal(configs); err != nil {
		return err
	} else {
		if dir, err := filepath.Abs(filepath.Dir(os.Args[0])); err != nil {
			return err
		} else {
			file := dir + "\\config\\serial.json"
			if runtime.GOOS != "windows" {
				file = dir + "/config/serial.json"
			}
			return ioutil.WriteFile(file, content, 0664)
		}
	}
}
