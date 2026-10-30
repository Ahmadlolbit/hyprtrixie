package ordered

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type member struct {
	key   string
	value json.RawMessage
}

type Object struct {
	members []member
}

func Parse(data []byte) (*Object, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if opening != json.Delim('{') {
		return nil, fmt.Errorf("not a JSON object: starts with %v", opening)
	}
	object := &Object{}
	for decoder.More() {
		if err := object.readMember(decoder); err != nil {
			return nil, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("not a JSON object: more text follows it")
	}
	return object, nil
}

func (object *Object) readMember(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	object.Set(fmt.Sprint(token), value)
	return nil
}

func (object *Object) Get(key string) (json.RawMessage, bool) {
	for _, found := range object.members {
		if found.key == key {
			return found.value, true
		}
	}
	return nil, false
}

func (object *Object) Set(key string, value json.RawMessage) {
	for index, found := range object.members {
		if found.key == key {
			object.members[index].value = value
			return
		}
	}
	object.members = append(object.members, member{key, value})
}

func (object *Object) Indented() ([]byte, error) {
	compact, err := object.compact()
	if err != nil {
		return nil, err
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, compact, "", "  "); err != nil {
		return nil, err
	}
	indented.WriteByte('\n')
	return indented.Bytes(), nil
}

func (object *Object) compact() ([]byte, error) {
	text := []byte{'{'}
	for index, found := range object.members {
		if index > 0 {
			text = append(text, ',')
		}
		key, err := json.Marshal(found.key)
		if err != nil {
			return nil, err
		}
		text = append(append(append(text, key...), ':'), found.value...)
	}
	return append(text, '}'), nil
}
