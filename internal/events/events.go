package events

import (
	"bufio"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const (
	Text     = "text"
	Rule     = "rule"
	OK       = "ok"
	Fail     = "fail"
	Skip     = "skip"
	Warn     = "warn"
	Busy     = "busy"
	Done     = "done"
	Note     = "note"
	Result   = "result"
	Page     = "page"
	Log      = "log"
	Choose   = "choose"
	Input    = "input"
	Password = "password"

	SummaryTitle = "Summary"

	waiting = 64
	answer  = "answer"
	cancel  = "cancel"
	stopped = "stop"
)

var kinds = []string{Text, Rule, OK, Fail, Skip, Warn, Busy, Done, Note, Result, Page, Log, Choose, Input, Password}

type Event struct {
	Kind  string
	Text  string
	Items []string
}

type reply struct {
	text string
	ok   bool
}

type Stream struct {
	mutex   sync.Mutex
	output  io.Writer
	replies chan reply
}

func Open(input io.Reader, output io.Writer, stop func()) *Stream {
	stream := &Stream{output: output, replies: make(chan reply, 1)}
	go stream.listen(input, stop)
	return stream
}

func parsed(line string) (reply, bool) {
	if line == cancel {
		return reply{}, true
	}
	quoted, found := strings.CutPrefix(line, answer+" ")
	if !found {
		return reply{}, false
	}
	text, err := strconv.Unquote(quoted)
	return reply{text, err == nil}, err == nil
}

func (stream *Stream) listen(input io.Reader, stop func()) {
	lines := bufio.NewReader(input)
	for {
		line, err := lines.ReadString('\n')
		if err != nil {
			stop()
			close(stream.replies)
			return
		}
		line = strings.TrimSuffix(line, "\n")
		if line == stopped {
			stop()
		} else if answered, known := parsed(line); known {
			stream.replies <- answered
		}
	}
}

func encoded(event Event) string {
	words := []string{event.Kind, strconv.Quote(event.Text)}
	for _, item := range event.Items {
		words = append(words, strconv.Quote(item))
	}
	return strings.Join(words, " ") + "\n"
}

func (stream *Stream) Send(event Event) {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()
	io.WriteString(stream.output, encoded(event))
}

func (stream *Stream) Ask(event Event) (string, bool) {
	stream.Send(event)
	answered, open := <-stream.replies
	return answered.text, open && answered.ok
}

func fields(text string) ([]string, bool) {
	var found []string
	for text != "" {
		quoted, err := strconv.QuotedPrefix(text)
		if err != nil {
			return nil, false
		}
		field, err := strconv.Unquote(quoted)
		if err != nil {
			return nil, false
		}
		found = append(found, field)
		text = strings.TrimPrefix(text[len(quoted):], " ")
	}
	return found, len(found) > 0
}

func decoded(line string) Event {
	kind, rest, _ := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
	words, ok := fields(rest)
	if !ok || !slices.Contains(kinds, kind) {
		return Event{Kind: Log, Text: line}
	}
	return Event{Kind: kind, Text: words[0], Items: words[1:]}
}

func Watch(output io.Reader) <-chan Event {
	found := make(chan Event, waiting)
	go func() {
		defer close(found)
		lines := bufio.NewReader(output)
		for {
			line, err := lines.ReadString('\n')
			if line != "" {
				found <- decoded(line)
			}
			if err != nil {
				return
			}
		}
	}()
	return found
}

func Answer(input io.Writer, text string, ok bool) error {
	line := cancel + "\n"
	if ok {
		line = answer + " " + strconv.Quote(text) + "\n"
	}
	_, err := io.WriteString(input, line)
	return err
}

func Stop(input io.Writer) error {
	_, err := io.WriteString(input, stopped+"\n")
	return err
}
