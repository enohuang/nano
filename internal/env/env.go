// Copyright (c) nano Authors. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// env represents the environment of the current process, includes
// work path and config path etc.
package env

import (
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gnano/serialize"
	"gnano/serialize/protobuf"
)

type ModeType uint32

const (
	NormalMode      ModeType = 0
	MaintenanceMode ModeType = 1
	DebugMode       ModeType = 2
)

var (
	Wd          string                   // working path
	Die         chan bool                // wait for end application
	Heartbeat   time.Duration            // Heartbeat internal
	CheckOrigin func(*http.Request) bool // check origin when websocket enabled
	Debug       bool                     // enable Debug
	// 0   正常模式  1 维护模式  2 调试模式
	Mode ModeType

	WSPath             string             // WebSocket path(eg: ws://127.0.0.1/WSPath)
	HandshakeValidator func([]byte) error // When you need to verify the custom data of the handshake request

	// timerPrecision indicates the precision of timer, default is time.Second
	TimerPrecision = time.Second

	// globalTicker represents global ticker that all cron job will be executed
	// in globalTicker.
	GlobalTicker *time.Ticker

	Serializer serialize.Serializer

	// conn count configurable
	ConnArrayMaxSize uint

	// GrpcOptions = []grpc.DialOption{grpc.WithInsecure()}
	GrpcOptions = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	SignalReload func()
	SignalQuit   func()
)

func init() {
	Die = make(chan bool)
	Heartbeat = 10 * time.Second
	ConnArrayMaxSize = 20
	Debug = false
	Mode = 0
	CheckOrigin = func(_ *http.Request) bool { return true }
	HandshakeValidator = func(_ []byte) error { return nil }
	Serializer = protobuf.NewSerializer()
	SignalQuit = func() {}
	SignalReload = func() {}
}
