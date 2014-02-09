package gotoc

import (
	"bytes"
	"code.google.com/p/go.tools/go/types"
	"io"
)

type DeclType int

const (
	FuncDecl DeclType = iota
	VarDecl
	ConstDecl
	TypeDecl
	ImportDecl
)

// CDD stores Go declaration translated to C declaration and definition.
type CDD struct {
	Origin     types.Object // object for this declaration
	DeclUses   map[types.Object]bool
	BodyUses   map[types.Object]bool
	Complexity int

	Typ    DeclType
	Export bool
	Inline bool // set by DetermineInline()

	Decl []byte
	Def  []byte
	Init []byte

	gtc  *GTC
	il   int  // indentation level
	body bool // true if translation process in function body
	dfsm int8
}

func (gtc *GTC) newCDD(o types.Object, t DeclType, il int) *CDD {
	cdd := &CDD{
		Origin:   o,
		Typ:      t,
		DeclUses: make(map[types.Object]bool),
		BodyUses: make(map[types.Object]bool),
		gtc:      gtc,
		il:       il,
		body:     il > 0,
	}
	return cdd
}

func (cdd *CDD) indent(w *bytes.Buffer) {
	for i := 0; i < cdd.il; i++ {
		w.WriteByte('\t')
	}
}

func (cdd *CDD) copyDecl(b *bytes.Buffer, suffix string) {
	n := b.Len()
	b.WriteString(suffix)
	cdd.Decl = append([]byte(nil), b.Bytes()...)
	b.Truncate(n)
}

func (cdd *CDD) copyDef(b *bytes.Buffer) {
	cdd.Def = append([]byte(nil), b.Bytes()...)
}

func (cdd *CDD) copyInit(b *bytes.Buffer) {
	cdd.Init = append([]byte(nil), b.Bytes()...)
}

func (cdd *CDD) WriteDecl(wh, wc io.Writer) error {
	if len(cdd.Decl) == 0 {
		return nil
	}

	prefix := ""

	switch cdd.Typ {
	case FuncDecl:
		if cdd.Inline {
			prefix = "static inline "
		} else if !cdd.Export && len(cdd.Def) > 0 {
			prefix = "static "
		}

	case VarDecl:
		if cdd.Export {
			prefix = "extern "
		} else {
			return nil
		}

	case ConstDecl:
		if !cdd.Export {
			return nil
		}
	}

	w := wc
	if cdd.Export {
		w = wh
	}

	_, err := io.WriteString(w, prefix)
	if err != nil {
		return err
	}
	_, err = w.Write(cdd.Decl)
	return err
}

func (cdd *CDD) WriteDef(wh, wc io.Writer) error {
	if len(cdd.Def) == 0 {
		return nil
	}

	prefix := ""
	w := wc

	switch cdd.Typ {
	case FuncDecl:
		if cdd.Export {
			if cdd.Inline {
				prefix = "static inline "
				w = wh
			}
		} else {
			prefix = "static "
		}

	case VarDecl:
		if !cdd.Export {
			prefix = "static "
		}

	case ConstDecl:
		return nil

	case TypeDecl:
		if cdd.Export {
			w = wh
		}
	}

	_, err := io.WriteString(w, prefix)
	if err != nil {
		return err
	}
	_, err = w.Write(cdd.Def)
	return err
}

func (cdd *CDD) DetermineInline() {
	if len(cdd.Def) == 0 {
		// Declaration only
		return
	}
	// TODO: Use more information (from il, BodyUses).
	// TODO: Complexity can be better calculated.
	if cdd.Complexity < 12 {
		cdd.Inline = true
	}
}

func (cdd *CDD) addObject(o types.Object, direct bool) {
	if o == cdd.Origin {
		return
	}
	if cdd.body {
		cdd.BodyUses[o] = direct
	} else {
		cdd.DeclUses[o] = direct
	}
}

func (cdd *CDD) dfs(all map[types.Object]*CDD, out []*CDD) []*CDD {
	if cdd.dfsm > 0 {
			panic("direct cycle in type declaration")
	}
	if cdd.dfsm < 0 {
			return out
	}
	cdd.dfsm = 1
	for o, direct := range cdd.DeclUses {
		if !direct {
			continue
		}
		u, ok := all[o]
		if !ok {
			continue
		}
		out = u.dfs(all, out)
	}
	cdd.dfsm = -1
	return append(out, cdd)
}

func dfs(all map[types.Object]*CDD) []*CDD {
	out := make([]*CDD, 0, len(all))
	for _, cdd := range all {
		out = cdd.dfs(all, out)
	}
	return out
}