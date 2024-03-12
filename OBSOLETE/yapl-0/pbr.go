/*
Copyright © 2024 Jeff Berkowitz (pdxjjb@gmail.com)

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package main

// pbbr.go - PushbackByteReader

import (
	"bufio"
	"io"
	"os"
	"strings"
)

type PushbackByteReader interface {
	io.ByteReader
	io.Closer
	UnreadByte(b byte)
}

type PBR struct {
	br io.ByteReader
	pb byte
}

func NewPathPushbackByteReader(path string) (PushbackByteReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return NewFilePushbackByteReader(f)
}

func NewFilePushbackByteReader(f *os.File) (PushbackByteReader, error) {
	return &PBR{br: bufio.NewReader(f), pb:0}, nil
}

func NewStringPushbackByteReader(body string) (PushbackByteReader, error) {
	return &PBR{br: strings.NewReader(body), pb:0}, nil
}

func (p *PBR) ReadByte() (byte, error) {
	if p.pb != 0 {
		result := p.pb
		p.pb = 0
		return result, nil
	}
	return p.br.ReadByte()
}

func (p *PBR) Close() error {
	closer, ok := p.br.(io.Closer)
	var result error = nil
	if ok {
		result = closer.Close()
	}
	return result
}

func (p *PBR) UnreadByte(b byte) {
	assert(b != 0, "PushbackByteReader: cannot pushback nul")
	assert(p.pb == 0, "PushbackByteReader: too many pushbacks")
	p.pb = b
}
