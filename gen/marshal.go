package gen

import (
	"fmt"
	"io"
	"os"

	"github.com/glycerine/zebrapack/cfg"
	"github.com/glycerine/zebrapack/msgp"
)

func marshal(w io.Writer, cfg *cfg.ZebraConfig) *marshalGen {
	return &marshalGen{
		p:   printer{w: w},
		cfg: cfg,
	}
}

type marshalGen struct {
	passes
	p    printer
	fuse []byte
	cfg  *cfg.ZebraConfig
}

func (m *marshalGen) Method() Method { return Marshal }

func (m *marshalGen) Apply(dirs []string) error {
	return nil
}

func (m *marshalGen) Execute(p Elem) error {
	if !m.p.ok() {
		return m.p.err
	}
	p = m.applyall(p)
	if p == nil {
		return nil
	}
	if !IsPrintable(p) {
		return nil
	}

	m.p.comment("MarshalMsg implements msgp.Marshaler")

	// save the vname before
	// calling methodReceiver so
	// that z.Msgsize() is printed correctly
	c := p.Varname()

	m.p.printf("\nfunc (%s %s) MarshalMsg(b []byte) (o []byte, err error) {", p.Varname(), imutMethodReceiver(p))
	m.p.printf("\no = msgp.Require(b, %s.Msgsize())", c)
	next(m, p)
	m.p.nakedReturn()
	return m.p.err
}

func (m *marshalGen) rawAppend(typ string, argfmt string, arg interface{}) {
	m.p.printf("\no = msgp.Append%s(o, %s)", typ, fmt.Sprintf(argfmt, arg))
}

func (m *marshalGen) fuseHook() {
	if len(m.fuse) > 0 {
		m.rawbytes(m.fuse)
		m.fuse = m.fuse[:0]
	}
}

func (m *marshalGen) Fuse(b []byte) {
	if len(m.fuse) == 0 {
		m.fuse = b
	} else {
		m.fuse = append(m.fuse, b...)
	}
}

func (m *marshalGen) gStruct(s *Struct) {
	if !m.p.ok() {
		return
	}

	if s.AsTuple {
		m.tuple(s)
	} else {
		m.mapstruct(s)
	}
	return
}

func (m *marshalGen) tuple(s *Struct) {
	data := make([]byte, 0, 5)
	data = msgp.AppendArrayHeader(data, uint32(len(s.Fields)-s.SkipCount))
	m.p.printf("\n// array header, size %d", len(s.Fields)-s.SkipCount)
	m.Fuse(data)
	for i := range s.Fields {
		if s.Fields[i].Skip {
			continue
		}
		if !m.p.ok() {
			return
		}
		next(m, s.Fields[i].FieldElem)
	}
}

func (m *marshalGen) mapstruct(s *Struct) {
	data := make([]byte, 0, 64)
	fast := !m.cfg.UseMsgp2
	nfields := len(s.Fields) - s.SkipCount
	// if fast, then always omit-empty.
	if fast || s.hasOmitEmptyTags {
		m.p.printf("\n\n// honor the omitempty tags\n")
		m.p.printf("var empty [%d]bool\n", len(s.Fields))
		m.p.printf("fieldsInUse := %s.fieldsNotEmpty(empty[:])\n", s.vname)
		if fast {
			// +1 for the -1:structName type-identifier.
			m.p.printf("	o = msgp.AppendMapHeader(o, fieldsInUse+1)\n")
		} else {
			m.p.printf("	o = msgp.AppendMapHeader(o, fieldsInUse)\n")
		}
	} else {
		data = msgp.AppendMapHeader(data, uint32(nfields))
		m.p.printf("\n// map header, size %d", nfields)
		m.Fuse(data)
	}

	if fast {
		// record the struct name under integer key -1
		recv := imutMethodReceiver(s)
		m.p.printf("\n// runtime struct type identification for '%s'\n", recv)
		hexname := ""
		for i := range recv {
			hexname += fmt.Sprintf("0x%x,", recv[i])
		}
		m.p.printf("o = msgp.AppendNegativeOneAndStringAsBytes(o, []byte{%s})\n", hexname)
	}

	for i := range s.Fields {
		if s.Fields[i].Skip {
			continue
		}
		if !m.p.ok() {
			return
		}
		if fast || (s.hasOmitEmptyTags && s.Fields[i].OmitEmpty) {
			m.p.printf("\n if !empty[%d] {", i)
		}

		if fast {
			// sanity check
			if s.Fields[i].ZebraId < 0 {
				fmt.Fprintf(os.Stderr, "\nzebrapack error: field '%s' is missing a zid number; cannot proceed under -fast\n", s.Fields[i].FieldTag)
				os.Exit(1)
			}
			// proceed

			data = msgp.AppendInt64(nil, s.Fields[i].ZebraId)
			m.p.printf("\n// zid %v for %q", s.Fields[i].ZebraId,
				s.Fields[i].FieldTag)
		} else {
			switch s.KeyTyp {
			case "Int64":
				data = msgp.AppendInt64(nil, s.Fields[i].ZebraId)
			default:
				data = msgp.AppendString(nil, s.Fields[i].FieldTag)
			}

			m.p.printf("\n// string %q", s.Fields[i].FieldTag)
		}
		m.Fuse(data)
		next(m, s.Fields[i].FieldElem)

		if fast || (s.hasOmitEmptyTags && s.Fields[i].OmitEmpty) {
			m.p.printf("\n }\n")
		}
	}
}

// append raw data
func (m *marshalGen) rawbytes(bts []byte) {
	m.p.print("\no = append(o, ")
	for _, b := range bts {
		m.p.printf("0x%x,", b)
	}
	m.p.print(")")
}

func (m *marshalGen) gMap(s *Map) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	vname := s.Varname()
	m.rawAppend(mapHeader, lenAsUint32, vname)
	m.p.printf("\nfor %s, %s := range %s {", s.Keyidx, s.Validx, vname)
	m.rawAppend(s.KeyTyp, literalFmt, s.Keyidx)
	next(m, s.Value)
	m.p.closeblock()
}

func (m *marshalGen) gSlice(s *Slice) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	vname := s.Varname()
	m.rawAppend(arrayHeader, lenAsUint32, vname)
	m.p.rangeBlock(s.Index, vname, m, s.Els)
}

func (m *marshalGen) gArray(a *Array) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	if be, ok := a.Els.(*BaseElem); ok && be.Value == Byte {
		m.rawAppend("Bytes", "%s[:]", a.Varname())
		return
	}

	m.rawAppend(arrayHeader, literalFmt, a.SizeResolved)
	m.p.rangeBlock(a.Index, a.Varname(), m, a.Els)
}

func (m *marshalGen) gPtr(p *Ptr) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	m.p.printf("\nif %s == nil {\no = msgp.AppendNil(o)\n} else {", p.Varname())
	next(m, p.Value)
	m.p.closeblock()
}

func (m *marshalGen) gBase(b *BaseElem) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	vname := b.Varname()

	if b.Convert {
		vname = tobaseConvert(b)
	}

	var echeck bool
	switch b.Value {
	case IDENT:
		echeck = true
		m.p.printf("\no, err = %s.MarshalMsg(o)", vname)
	case Intf, Ext:
		echeck = true
		m.p.printf("\no, err = msgp.Append%s(o, %s)", b.BaseName(), vname)
	default:
		m.rawAppend(b.BaseName(), literalFmt, vname)
	}

	if echeck {
		m.p.print(errcheck)
	}
}
