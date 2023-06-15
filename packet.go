package forward

import (
	"encoding/json"
	"io"
)

type Packet struct {
	Cmd  string
	Data map[string]string
}

func (p *Packet) Encode(w io.Writer) error {
	return json.NewEncoder(w).Encode(p)
}

func (p *Packet) Decode(r io.Reader) error {
	return json.NewDecoder(r).Decode(p)
}
