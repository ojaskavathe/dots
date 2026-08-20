//go:build !noequalize

package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"time"
)

// A minimal msgpack-rpc client for nvim — just enough for equalize's two
// calls (nvim_eval, nvim_command), so demuxd stays dependency-free. Results
// that need structure are fetched as json_encode(...) strings and parsed
// with encoding/json; the msgpack side only ever decodes the reply envelope.

type nvimConn struct {
	c     net.Conn
	br    *bufio.Reader
	msgid uint32
}

func nvimDial(path string) (*nvimConn, error) {
	c, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	return &nvimConn{c: c, br: bufio.NewReader(c)}, nil
}

func (n *nvimConn) Close() { _ = n.c.Close() }

// EvalJSON evaluates expr inside json_encode() and returns the JSON text.
func (n *nvimConn) EvalJSON(expr string) (string, error) {
	res, err := n.call("nvim_eval", "json_encode("+expr+")")
	if err != nil {
		return "", err
	}
	s, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("nvim_eval: non-string result %T", res)
	}
	return s, nil
}

func (n *nvimConn) Command(cmd string) error {
	_, err := n.call("nvim_command", cmd)
	return err
}

// call sends [0, msgid, method, [arg]] and waits for the matching
// [1, msgid, error, result], skipping any notifications in between.
func (n *nvimConn) call(method, arg string) (any, error) {
	n.msgid++
	id := n.msgid
	var buf []byte
	buf = append(buf, 0x94, 0x00)                   // fixarray(4), type request
	buf = append(buf, 0xce)                         // uint32 msgid
	buf = binary.BigEndian.AppendUint32(buf, id)    //
	buf = append(buf, 0xa0|byte(len(method)))       // fixstr method (always short)
	buf = append(buf, method...)                    //
	buf = append(buf, 0x91)                         // fixarray(1) args
	buf = mpAppendStr(buf, arg)                     //
	deadline := time.Now().Add(2 * time.Second)
	_ = n.c.SetDeadline(deadline)
	if _, err := n.c.Write(buf); err != nil {
		return nil, err
	}
	for {
		v, err := mpDecode(n.br)
		if err != nil {
			return nil, err
		}
		msg, ok := v.([]any)
		if !ok || len(msg) < 3 {
			return nil, fmt.Errorf("nvim rpc: bad message %T", v)
		}
		kind, _ := msg[0].(int64)
		if kind == 2 {
			continue // notification; not ours
		}
		if kind != 1 || len(msg) != 4 {
			return nil, fmt.Errorf("nvim rpc: unexpected message type %v", msg[0])
		}
		if got, _ := msg[1].(int64); uint32(got) != id {
			continue // reply to someone else's request; keep waiting
		}
		if msg[2] != nil {
			return nil, fmt.Errorf("nvim rpc: %v", msg[2])
		}
		return msg[3], nil
	}
}

func mpAppendStr(buf []byte, s string) []byte {
	switch l := len(s); {
	case l < 32:
		buf = append(buf, 0xa0|byte(l))
	case l < 256:
		buf = append(buf, 0xd9, byte(l))
	case l < 65536:
		buf = append(buf, 0xda)
		buf = binary.BigEndian.AppendUint16(buf, uint16(l))
	default:
		buf = append(buf, 0xdb)
		buf = binary.BigEndian.AppendUint32(buf, uint32(l))
	}
	return append(buf, s...)
}

// mpDecode reads one msgpack value: ints normalize to int64, str to string,
// arrays to []any, maps to map[any]any, ext types to raw bytes (nvim uses
// ext for Buffer/Window/Tabpage handles — opaque here, never inspected).
func mpDecode(r *bufio.Reader) (any, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	switch {
	case b <= 0x7f:
		return int64(b), nil
	case b >= 0xe0:
		return int64(int8(b)), nil
	case b >= 0xa0 && b <= 0xbf:
		return mpReadStr(r, int(b&0x1f))
	case b >= 0x90 && b <= 0x9f:
		return mpReadArray(r, int(b&0x0f))
	case b >= 0x80 && b <= 0x8f:
		return mpReadMap(r, int(b&0x0f))
	}
	switch b {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xcc, 0xcd, 0xce, 0xcf: // uint 8/16/32/64
		n, err := mpReadUint(r, 1<<(b-0xcc))
		return int64(n), err
	case 0xd0, 0xd1, 0xd2, 0xd3: // int 8/16/32/64
		n, err := mpReadUint(r, 1<<(b-0xd0))
		switch b {
		case 0xd0:
			return int64(int8(n)), err
		case 0xd1:
			return int64(int16(n)), err
		case 0xd2:
			return int64(int32(n)), err
		}
		return int64(n), err
	case 0xca: // float32: read and discard precision distinction
		n, err := mpReadUint(r, 4)
		return float64(math.Float32frombits(uint32(n))), err
	case 0xcb:
		n, err := mpReadUint(r, 8)
		return math.Float64frombits(n), err
	case 0xd9, 0xda, 0xdb: // str 8/16/32
		l, err := mpReadUint(r, 1<<(b-0xd9))
		if err != nil {
			return nil, err
		}
		return mpReadStr(r, int(l))
	case 0xc4, 0xc5, 0xc6: // bin 8/16/32
		l, err := mpReadUint(r, 1<<(b-0xc4))
		if err != nil {
			return nil, err
		}
		return mpReadBytes(r, int(l))
	case 0xdc, 0xdd: // array 16/32
		l, err := mpReadUint(r, 2<<(b-0xdc))
		if err != nil {
			return nil, err
		}
		return mpReadArray(r, int(l))
	case 0xde, 0xdf: // map 16/32
		l, err := mpReadUint(r, 2<<(b-0xde))
		if err != nil {
			return nil, err
		}
		return mpReadMap(r, int(l))
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8: // fixext 1/2/4/8/16
		return mpReadBytes(r, 1+(1<<(b-0xd4)))
	case 0xc7, 0xc8, 0xc9: // ext 8/16/32
		l, err := mpReadUint(r, 1<<(b-0xc7))
		if err != nil {
			return nil, err
		}
		return mpReadBytes(r, int(l)+1)
	}
	return nil, fmt.Errorf("msgpack: unhandled byte %#x", b)
}

func mpReadUint(r *bufio.Reader, n int) (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:n]); err != nil {
		return 0, err
	}
	v := uint64(0)
	for i := 0; i < n; i++ {
		v = v<<8 | uint64(buf[i])
	}
	return v, nil
}

func mpReadBytes(r *bufio.Reader, n int) ([]byte, error) {
	if n < 0 || n > 1<<24 {
		return nil, errors.New("msgpack: unreasonable length")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func mpReadStr(r *bufio.Reader, n int) (string, error) {
	b, err := mpReadBytes(r, n)
	return string(b), err
}

func mpReadArray(r *bufio.Reader, n int) ([]any, error) {
	if n < 0 || n > 1<<20 {
		return nil, errors.New("msgpack: unreasonable array")
	}
	out := make([]any, n)
	for i := range out {
		v, err := mpDecode(r)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func mpReadMap(r *bufio.Reader, n int) (map[any]any, error) {
	if n < 0 || n > 1<<20 {
		return nil, errors.New("msgpack: unreasonable map")
	}
	out := make(map[any]any, n)
	for i := 0; i < n; i++ {
		k, err := mpDecode(r)
		if err != nil {
			return nil, err
		}
		v, err := mpDecode(r)
		if err != nil {
			return nil, err
		}
		switch k.(type) {
		case string, int64, bool, float64:
			out[k] = v
		}
	}
	return out, nil
}
