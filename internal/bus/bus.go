package bus

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	limit            = 2 * time.Second
	largest          = 1 << 20
	tries            = 16
	daemon           = "org.freedesktop.DBus"
	daemonPath       = "/org/freedesktop/DBus"
	methodCall       = 1
	methodError      = 3
	fieldPath        = 1
	fieldInterface   = 2
	fieldMember      = 3
	fieldReplySerial = 5
	fieldDestination = 6
	fieldSignature   = 8
	listSerial       = 2
)

var (
	errNoBus   = errors.New("no session bus")
	errRefused = errors.New("the session bus didn't list its names")
	errGarbled = errors.New("the session bus sent something unreadable")
	orders     = map[byte]binary.ByteOrder{'l': binary.LittleEndian, 'B': binary.BigEndian}
	options    = map[string]string{"path": "", "abstract": "@"}
)

type message struct {
	kind      byte
	replyTo   uint32
	signature string
	body      []byte
	order     binary.ByteOrder
}

type decoder struct {
	data  []byte
	at    int
	order binary.ByteOrder
	bad   bool
}

func Names() ([]string, error) {
	path, err := address()
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("unix", path, limit)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(limit)); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	if err := authenticate(conn, reader); err != nil {
		return nil, err
	}
	request := append([]byte("BEGIN\r\n"), call(1, "Hello")...)
	if _, err := conn.Write(append(request, call(listSerial, "ListNames")...)); err != nil {
		return nil, err
	}
	reply, err := answer(reader, listSerial)
	if err != nil {
		return nil, err
	}
	return reply.names()
}

func address() (string, error) {
	value := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if value == "" {
		return runtimeBus()
	}
	for _, entry := range strings.Split(value, ";") {
		if path, found := unixPath(entry); found {
			return path, nil
		}
	}
	return "", errNoBus
}

func runtimeBus() (string, error) {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		return "", errNoBus
	}
	return filepath.Join(runtime, "bus"), nil
}

func unixPath(entry string) (string, bool) {
	transport, pairs, _ := strings.Cut(entry, ":")
	if transport != "unix" {
		return "", false
	}
	for _, pair := range strings.Split(pairs, ",") {
		key, value, _ := strings.Cut(pair, "=")
		prefix, known := options[key]
		path, err := url.PathUnescape(value)
		if known && err == nil && path != "" {
			return prefix + path, true
		}
	}
	return "", false
}

func authenticate(writer io.Writer, reader *bufio.Reader) error {
	user := hex.EncodeToString([]byte(strconv.Itoa(os.Getuid())))
	if _, err := writer.Write([]byte("\x00AUTH EXTERNAL " + user + "\r\n")); err != nil {
		return err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "OK ") {
		return fmt.Errorf("the session bus said %q", strings.TrimSpace(line))
	}
	return nil
}

func call(serial uint32, member string) []byte {
	var fields []byte
	for _, field := range []struct {
		code  byte
		kind  byte
		value string
	}{{fieldPath, 'o', daemonPath}, {fieldInterface, 's', daemon}, {fieldMember, 's', member}, {fieldDestination, 's', daemon}} {
		fields = padded(fields, 8)
		fields = append(fields, field.code, 1, field.kind, 0)
		fields = binary.LittleEndian.AppendUint32(fields, uint32(len(field.value)))
		fields = append(append(fields, field.value...), 0)
	}
	head := []byte{'l', methodCall, 0, 1, 0, 0, 0, 0}
	head = binary.LittleEndian.AppendUint32(head, serial)
	head = binary.LittleEndian.AppendUint32(head, uint32(len(fields)))
	return padded(append(head, fields...), 8)
}

func padded(data []byte, size int) []byte {
	for len(data)%size != 0 {
		data = append(data, 0)
	}
	return data
}

func answer(reader io.Reader, serial uint32) (message, error) {
	for range tries {
		got, err := receive(reader)
		if err != nil {
			return message{}, err
		}
		if got.replyTo == serial {
			return got, nil
		}
	}
	return message{}, errGarbled
}

func receive(reader io.Reader) (message, error) {
	head := make([]byte, 16)
	if _, err := io.ReadFull(reader, head); err != nil {
		return message{}, err
	}
	order, known := orders[head[0]]
	if !known {
		return message{}, errGarbled
	}
	fields, size := order.Uint32(head[12:]), order.Uint32(head[4:8])
	if fields > largest || size > largest {
		return message{}, errGarbled
	}
	start := int(fields+7) &^ 7
	rest := make([]byte, start+int(size))
	if _, err := io.ReadFull(reader, rest); err != nil {
		return message{}, err
	}
	header := &decoder{data: rest[:fields], order: order}
	got := message{kind: head[1], body: rest[start:], order: order}
	got.replyTo, got.signature = header.fields()
	if header.bad {
		return message{}, errGarbled
	}
	return got, nil
}

func (m message) names() ([]string, error) {
	if m.kind == methodError {
		return nil, errRefused
	}
	if m.signature != "as" {
		return nil, errGarbled
	}
	body := &decoder{data: m.body, order: m.order}
	end := int(body.number()) + body.at
	var names []string
	for !body.bad && body.at < end {
		names = append(names, body.text())
	}
	if body.bad {
		return nil, errGarbled
	}
	return names, nil
}

func (d *decoder) fields() (uint32, string) {
	var serial uint32
	var signature string
	for !d.bad && d.at < len(d.data) {
		d.align(8)
		code := d.octet()
		switch d.signature() {
		case "u":
			if number := d.number(); code == fieldReplySerial {
				serial = number
			}
		case "s", "o":
			d.text()
		case "g":
			if text := d.signature(); code == fieldSignature {
				signature = text
			}
		default:
			d.bad = true
		}
	}
	return serial, signature
}

func (d *decoder) take(size int) []byte {
	if d.bad || size < 0 || size > len(d.data)-d.at {
		d.bad = true
		return nil
	}
	part := d.data[d.at : d.at+size]
	d.at += size
	return part
}

func (d *decoder) align(size int) {
	d.take((size - d.at%size) % size)
}

func (d *decoder) octet() byte {
	if part := d.take(1); part != nil {
		return part[0]
	}
	return 0
}

func (d *decoder) number() uint32 {
	d.align(4)
	if part := d.take(4); part != nil {
		return d.order.Uint32(part)
	}
	return 0
}

func (d *decoder) text() string {
	size := d.number()
	if part := d.take(int(size) + 1); part != nil {
		return string(part[:size])
	}
	return ""
}

func (d *decoder) signature() string {
	size := d.octet()
	if part := d.take(int(size) + 1); part != nil {
		return string(part[:size])
	}
	return ""
}
