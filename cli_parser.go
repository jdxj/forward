package forward

import (
	"bufio"
	"io"
	"net/url"
	"strings"
)

func NewCliParser(r io.Reader, w io.Writer) *CliParser {
	c := &CliParser{
		r: bufio.NewReaderSize(r, 1024),
		w: bufio.NewWriterSize(w, 1024),
	}
	return c
}

type CliParser struct {
	r *bufio.Reader
	w *bufio.Writer
}

func (c *CliParser) Decode() (*Packet, error) {
	line, err := c.r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimRight(line, "\r\n")

	args, err := url.ParseQuery(line)
	if err != nil {
		return nil, err
	}

	p := &Packet{
		Cmd:  args.Get("cmd"),
		Data: make(map[string]string),
	}

	for k, vs := range args {
		if k == "cmd" || len(vs) == 0 {
			continue
		}
		p.Data[k] = vs[0]
	}
	return p, nil
}

func (c *CliParser) Encode(p *Packet) error {
	if p == nil {
		return nil
	}
	// cmd=abc&k=v
	args := url.Values{}
	if p.Cmd != "" {
		args.Set("cmd", p.Cmd)
	}
	for k, v := range p.Data {
		args.Set(k, v)
	}
	argsStr, err := url.QueryUnescape(args.Encode())
	if err != nil {
		return err
	}

	_, _ = c.w.WriteString(argsStr + "\n")
	return c.w.Flush()
}
