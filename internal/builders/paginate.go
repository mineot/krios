package builders

import (
	"strconv"
	"strings"
)

type Paginate struct {
	Limit  uint16
	Offset uint16
}

func (p *Paginate) IsEmpty() bool {
	return p.Limit == 0 && p.Offset == 0
}

func (p *Paginate) Next() {
	p.Offset += p.Limit
}

func (p *Paginate) Previus() {
	if p.Offset > 0 {
		p.Offset -= p.Limit
	}
}

func (p *Paginate) Build() string {
	w := strings.Builder{}
	w.WriteString(" LIMIT ")
	w.WriteString(strconv.FormatUint(uint64(p.Limit), 10))
	w.WriteString(" OFFSET ")
	w.WriteString(strconv.FormatUint(uint64(p.Offset), 10))
	return w.String()
}
