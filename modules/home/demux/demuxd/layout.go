package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Mini layout engine for docked-mode give-back. When the user reshapes a
// window WHILE the sidebar is docked (tmux-equalize-nvim sets
// @demux_layout_dirty), restoring the pre-dock snapshot on leave would undo
// their change. Instead: parse the docked #{window_layout}, drop the sidebar
// leaf, rescale what remains to the full window width, and apply that.
// Heights are untouched — the sidebar only ever takes width.
//
// Parser/renderer mirror tmux's layout_dump format (and the copy in
// tmux-equalize-nvim): WxH,X,Y then ",pane" for a leaf or {..}/[..] for
// horizontal/vertical splits. select-layout assigns cells to panes by list
// order, so this is only ever applied AFTER the sidebar has left the window,
// when list order matches the string's geometric order again.

type lnode struct {
	kind byte // 'l' leaf, '{' row, '[' column
	w, h int
	x, y int
	pane string
	kids []*lnode
}

type lparser struct {
	s string
	i int
}

func (p *lparser) num() (int, error) {
	start := p.i
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
	}
	if start == p.i {
		return 0, fmt.Errorf("layout: number expected at %d", p.i)
	}
	return strconv.Atoi(p.s[start:p.i])
}

func (p *lparser) expect(c byte) error {
	if p.i >= len(p.s) || p.s[p.i] != c {
		return fmt.Errorf("layout: %q expected at %d", c, p.i)
	}
	p.i++
	return nil
}

func (p *lparser) node() (*lnode, error) {
	n := &lnode{}
	var err error
	if n.w, err = p.num(); err != nil {
		return nil, err
	}
	if err = p.expect('x'); err != nil {
		return nil, err
	}
	if n.h, err = p.num(); err != nil {
		return nil, err
	}
	if err = p.expect(','); err != nil {
		return nil, err
	}
	if n.x, err = p.num(); err != nil {
		return nil, err
	}
	if err = p.expect(','); err != nil {
		return nil, err
	}
	if n.y, err = p.num(); err != nil {
		return nil, err
	}
	if p.i >= len(p.s) {
		return nil, errors.New("layout: truncated")
	}
	switch c := p.s[p.i]; c {
	case ',':
		p.i++
		id, err := p.num()
		if err != nil {
			return nil, err
		}
		n.kind = 'l'
		n.pane = strconv.Itoa(id)
		return n, nil
	case '{', '[':
		p.i++
		n.kind = c
		end := byte('}')
		if c == '[' {
			end = ']'
		}
		for {
			kid, err := p.node()
			if err != nil {
				return nil, err
			}
			n.kids = append(n.kids, kid)
			if p.i >= len(p.s) {
				return nil, errors.New("layout: unterminated split")
			}
			if p.s[p.i] == end {
				p.i++
				return n, nil
			}
			if err := p.expect(','); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("layout: unexpected %q at %d", c, p.i)
	}
}

func lrender(n *lnode) string {
	head := fmt.Sprintf("%dx%d,%d,%d", n.w, n.h, n.x, n.y)
	if n.kind == 'l' {
		return head + "," + n.pane
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteByte(n.kind)
	for i, kid := range n.kids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(lrender(kid))
	}
	if n.kind == '{' {
		b.WriteByte('}')
	} else {
		b.WriteByte(']')
	}
	return b.String()
}

// lchecksum is tmux's layout checksum (same as layout_checksum).
func lchecksum(body string) string {
	var sum uint16
	for _, r := range body {
		sum = (sum >> 1) + ((sum & 1) << 15)
		sum += uint16(r)
	}
	return fmt.Sprintf("%04x", sum)
}

// scaleX resizes a subtree to a new x/width, proportionally, leaving
// vertical geometry alone.
func scaleX(n *lnode, x, w int) {
	n.x, n.w = x, w
	switch n.kind {
	case '[':
		for _, kid := range n.kids {
			scaleX(kid, x, w)
		}
	case '{':
		seps := len(n.kids) - 1
		avail := w - seps
		old := 0
		for _, kid := range n.kids {
			old += kid.w
		}
		if old <= 0 || avail < len(n.kids) {
			avail = max(avail, len(n.kids))
			old = len(n.kids)
		}
		// largest-remainder proportional allocation
		widths := make([]int, len(n.kids))
		rem := make([]float64, len(n.kids))
		total := 0
		for i, kid := range n.kids {
			exact := float64(avail) * float64(kid.w) / float64(old)
			widths[i] = max(1, int(exact))
			rem[i] = exact - float64(int(exact))
			total += widths[i]
		}
		for total < avail {
			best := 0
			for i := range rem {
				if rem[i] > rem[best] {
					best = i
				}
			}
			widths[best]++
			rem[best] = -1
			total++
		}
		for total > avail {
			big := 0
			for i := range widths {
				if widths[i] > widths[big] {
					big = i
				}
			}
			if widths[big] <= 1 {
				break
			}
			widths[big]--
			total--
		}
		cx := x
		for i, kid := range n.kids {
			scaleX(kid, cx, widths[i])
			cx += widths[i] + 1
		}
	}
}

// sansSidebar takes a docked window's layout (checksum,body) and the sidebar
// pane id (%N), removes the sidebar leaf, and rescales the remaining panes
// across the full window width. Returns a complete checksum,body string for
// select-layout.
func sansSidebar(layout, sidebarPane string) (string, error) {
	_, body, ok := strings.Cut(layout, ",")
	if !ok {
		return "", fmt.Errorf("layout: no checksum in %q", layout)
	}
	root, err := (&lparser{s: body}).node()
	if err != nil {
		return "", err
	}
	id := strings.TrimPrefix(sidebarPane, "%")
	if root.kind != '{' {
		return "", errors.New("layout: root is not a row; sidebar not found")
	}
	idx := -1
	for i, kid := range root.kids {
		if kid.kind == 'l' && kid.pane == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", errors.New("layout: sidebar is not a direct root child")
	}
	kids := append(append([]*lnode{}, root.kids[:idx]...), root.kids[idx+1:]...)
	if len(kids) == 0 {
		return "", errors.New("layout: nothing left without the sidebar")
	}
	if len(kids) == 1 {
		n := kids[0]
		scaleX(n, root.x, root.w)
		n.y, n.h = root.y, root.h
		body = lrender(n)
		return lchecksum(body) + "," + body, nil
	}
	root.kids = kids
	scaleX(root, root.x, root.w)
	body = lrender(root)
	return lchecksum(body) + "," + body, nil
}
