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
	"bytes"
	"fmt"
	"strings"
	"testing"
	"vilan/netty"
	"vilan/netty/utils"
)

func TestTextCodec(t *testing.T) {

	var cases = []struct {
		input  []byte
		output interface{}
	}{
		{output: "123456789"},
		{output: strings.NewReader("123456789")},
	}

	for index, c := range cases {
		codec := TextCodec()
		t.Run(fmt.Sprint(codec.CodecName(), "#", index), func(t *testing.T) {
			ctx := MockHandlerContext{
				MockHandleRead: func(message netty.Message) {
					if dst := utils.MustToBytes(message); !bytes.Equal(dst, c.input) {
						t.Fatalf("%v != %v", dst, c.input)
					}
				},

				MockHandleWrite: func(message netty.Message) {
					c.input = utils.MustToBytes(message)
				},
			}
			codec.HandleWrite(ctx, c.output)
			codec.HandleRead(ctx, c.input)
		})
	}
}
