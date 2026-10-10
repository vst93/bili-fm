package ui

import (
	"io"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"
)

// TextBuffer stores editable UTF-8 text in an indexed, persistent tree.
// Small edits copy bounded chunks and tree paths, not the whole document.
// Rune offsets are used by Slice/Replace, matching Element.TextSelection;
// UTF16Offset/RuneOffset and TextForRange bridge native input coordinates.
// The zero value is an empty buffer. Methods are safe from any goroutine.
// Schedule edits with Window.Update to invalidate a view showing the buffer.
type TextBuffer struct {
	mu      sync.RWMutex
	root    *textNode
	version uint64
}

// TextSnapshot is an immutable, inexpensive view of a buffer. It shares
// unchanged chunks and can be read concurrently with further edits.
type TextSnapshot struct {
	root    *textNode
	version uint64
}

// NewTextBuffer creates indexed storage owning its text chunks. A small
// surviving fragment cannot pin the original large input string.
func NewTextBuffer(s string) *TextBuffer { return &TextBuffer{root: textTree(s)} }

func (b *TextBuffer) Snapshot() TextSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return TextSnapshot{root: b.root, version: b.version}
}

func (b *TextBuffer) Len() int        { return b.Snapshot().Len() }
func (b *TextBuffer) ByteLen() int    { return b.Snapshot().ByteLen() }
func (b *TextBuffer) UTF16Len() int   { return b.Snapshot().UTF16Len() }
func (b *TextBuffer) LineCount() int  { return b.Snapshot().LineCount() }
func (b *TextBuffer) Version() uint64 { return b.Snapshot().version }

// String materializes the document for export. Controls do not call it
// during ordinary editing; Slice and WriteTo avoid that full allocation.
func (b *TextBuffer) String() string              { return b.Snapshot().String() }
func (b *TextBuffer) Slice(start, end int) string { return b.Snapshot().Slice(start, end) }
func (b *TextBuffer) UTF16Offset(index int) int   { return b.Snapshot().UTF16Offset(index) }
func (b *TextBuffer) RuneOffset(index int) int    { return b.Snapshot().RuneOffset(index) }
func (b *TextBuffer) TextForRange(r TextInputRange) (string, TextInputRange) {
	return b.Snapshot().TextForRange(r)
}
func (b *TextBuffer) WriteTo(w io.Writer) (int64, error) { return b.Snapshot().WriteTo(w) }

// Set replaces the document. Existing snapshots retain their own version.
func (b *TextBuffer) Set(s string) { b.Restore(TextSnapshot{root: textTree(s)}) }

// Restore adopts a snapshot in constant time and advances the version.
func (b *TextBuffer) Restore(s TextSnapshot) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.root = s.root
	b.version++
}

// Replace replaces a half-open rune range, clamping it to the document.
func (b *TextBuffer) Replace(start, end int, s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.root = textReplace(b.root, start, end, s)
	b.version++
}

// ReplaceUTF16 adjusts surrogate interiors down to whole runes, replaces
// the range, and returns the actual old UTF-16 range it replaced.
func (b *TextBuffer) ReplaceUTF16(r TextInputRange, s string) TextInputRange {
	b.mu.Lock()
	defer b.mu.Unlock()
	view := TextSnapshot{root: b.root}
	a, z := view.RuneOffset(min(r.Start, r.End)), view.RuneOffset(max(r.Start, r.End))
	actual := TextInputRange{Start: view.UTF16Offset(a), End: view.UTF16Offset(z)}
	b.root = textReplace(b.root, a, z, s)
	b.version++
	return actual
}

// compareRestore prevents a widget from overwriting a concurrent program edit.
func (b *TextBuffer) compareRestore(before TextSnapshot, root *textNode) (TextSnapshot, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.root != before.root || b.version != before.version {
		return TextSnapshot{root: b.root, version: b.version}, false
	}
	b.root = root
	b.version++
	return TextSnapshot{root: root, version: b.version}, true
}

func (s TextSnapshot) Len() int        { return textSum(s.root).runes }
func (s TextSnapshot) ByteLen() int    { return textSum(s.root).bytes }
func (s TextSnapshot) UTF16Len() int   { return textSum(s.root).units }
func (s TextSnapshot) LineCount() int  { return textSum(s.root).lines + 1 }
func (s TextSnapshot) Version() uint64 { return s.version }
func (s TextSnapshot) String() string  { return s.Slice(0, s.Len()) }
func (s TextSnapshot) Slice(start, end int) string {
	a, z := max(0, min(start, end, s.Len())), max(0, min(max(start, end), s.Len()))
	lo, hi := textByteAt(s.root, a), textByteAt(s.root, z)
	var out strings.Builder
	out.Grow(hi - lo)
	textRead(s.root, lo, hi, &out)
	return out.String()
}
func (s TextSnapshot) UTF16Offset(index int) int {
	return textPrefix(s.root, textByteAt(s.root, index)).units
}
func (s TextSnapshot) RuneOffset(index int) int {
	return textRuneAtUnits(s.root, max(0, min(index, s.UTF16Len())))
}
func (s TextSnapshot) TextForRange(r TextInputRange) (string, TextInputRange) {
	a, z := s.RuneOffset(min(r.Start, r.End)), s.RuneOffset(max(r.Start, r.End))
	return nativeText(s.Slice(a, z)), TextInputRange{Start: s.UTF16Offset(a), End: s.UTF16Offset(z)}
}

// LineRange returns a line's rune offsets, excluding its following newline.
// The terminal empty line after a newline is counted; line indices clamp.
func (s TextSnapshot) LineRange(line int) (int, int) {
	line = max(0, min(line, s.LineCount()-1))
	a := textPrefix(s.root, textLineStart(s.root, line)).runes
	z := s.Len()
	if line+1 < s.LineCount() {
		z = textPrefix(s.root, textLineStart(s.root, line+1)).runes - 1
	}
	return a, z
}
func (s TextSnapshot) Line(line int) string { a, z := s.LineRange(line); return s.Slice(a, z) }
func (s TextSnapshot) LineAt(index int) int {
	return textPrefix(s.root, textByteAt(s.root, index)).lines
}

func (s TextSnapshot) WriteTo(w io.Writer) (int64, error) {
	var total int64
	var visit func(*textNode) error
	visit = func(n *textNode) error {
		if n == nil {
			return nil
		}
		if n.left == nil {
			count, err := io.WriteString(w, n.text)
			total += int64(count)
			if err == nil && count != len(n.text) {
				err = io.ErrShortWrite
			}
			return err
		}
		if err := visit(n.left); err != nil {
			return err
		}
		return visit(n.right)
	}
	err := visit(s.root)
	return total, err
}

const textChunkBytes = 4096

type textSummary struct{ bytes, runes, units, lines int }
type textNode struct {
	left, right *textNode
	text        string // only leaves own bytes
	sum         textSummary
	height      int
}

func textSum(n *textNode) textSummary {
	if n == nil {
		return textSummary{}
	}
	return n.sum
}
func textHeight(n *textNode) int {
	if n == nil {
		return 0
	}
	return n.height
}
func addText(a, b textSummary) textSummary {
	return textSummary{a.bytes + b.bytes, a.runes + b.runes, a.units + b.units, a.lines + b.lines}
}
func summarizeText(s string) textSummary {
	n := countRunes(s)
	return textSummary{len(s), n, unitsForRunes(s, n), strings.Count(s, "\n")}
}
func textLeaf(s string) *textNode {
	if s == "" {
		return nil
	}
	return &textNode{text: strings.Clone(s), sum: summarizeText(s), height: 1}
}
func textBranch(a, b *textNode) *textNode {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return &textNode{left: a, right: b, sum: addText(a.sum, b.sum), height: 1 + max(a.height, b.height)}
}
func textBalance(n *textNode) *textNode {
	if textHeight(n.left)-textHeight(n.right) > 1 {
		a := n.left
		if textHeight(a.left) < textHeight(a.right) {
			b := a.right
			a = textBranch(textBranch(a.left, b.left), b.right)
		}
		return textBranch(a.left, textBranch(a.right, n.right))
	}
	if textHeight(n.right)-textHeight(n.left) > 1 {
		b := n.right
		if textHeight(b.right) < textHeight(b.left) {
			a := b.left
			b = textBranch(a.left, textBranch(a.right, b.right))
		}
		return textBranch(textBranch(n.left, b.left), b.right)
	}
	return n
}
func textJoin(a, b *textNode) *textNode {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.left == nil && b.left == nil && a.sum.bytes+b.sum.bytes <= textChunkBytes {
		return textLeaf(a.text + b.text)
	}
	if a.height > b.height+1 {
		return textBalance(textBranch(a.left, textJoin(a.right, b)))
	}
	if b.height > a.height+1 {
		return textBalance(textBranch(textJoin(a, b.left), b.right))
	}
	return textBranch(a, b)
}
func textTree(s string) *textNode {
	var leaves []*textNode
	for start := 0; start < len(s); {
		end := start
		for end < len(s) && end-start < textChunkBytes {
			_, n := utf8.DecodeRuneInString(s[end:])
			end += n
		}
		leaves = append(leaves, textLeaf(s[start:end]))
		start = end
	}
	var build func(int, int) *textNode
	build = func(a, z int) *textNode {
		if a == z {
			return nil
		}
		if z-a == 1 {
			return leaves[a]
		}
		m := (a + z) / 2
		return textBranch(build(a, m), build(m, z))
	}
	return build(0, len(leaves))
}
func textSplit(n *textNode, index int) (*textNode, *textNode) {
	if n == nil {
		return nil, nil
	}
	if index <= 0 {
		return nil, n
	}
	if index >= n.sum.bytes {
		return n, nil
	}
	if n.left == nil {
		return textLeaf(n.text[:index]), textLeaf(n.text[index:])
	}
	if index < n.left.sum.bytes {
		a, b := textSplit(n.left, index)
		return a, textJoin(b, n.right)
	}
	a, b := textSplit(n.right, index-n.left.sum.bytes)
	return textJoin(n.left, a), b
}
func textReplace(n *textNode, start, end int, s string) *textNode {
	total := textSum(n).runes
	a, z := max(0, min(start, end, total)), max(0, min(max(start, end), total))
	lo, hi := textByteAt(n, a), textByteAt(n, z)
	left, rest := textSplit(n, lo)
	_, right := textSplit(rest, hi-lo)
	return textJoin(textJoin(left, textTree(s)), right)
}
func textByteAt(n *textNode, index int) int {
	index = max(0, min(index, textSum(n).runes))
	offset := 0
	for n != nil && n.left != nil {
		if index < n.left.sum.runes {
			n = n.left
		} else {
			index -= n.left.sum.runes
			offset += n.left.sum.bytes
			n = n.right
		}
	}
	if n == nil {
		return offset
	}
	if n.sum.bytes == n.sum.runes {
		return offset + index
	}
	return offset + runeOffset(n.text, index)
}
func textRuneAtUnits(n *textNode, index int) int {
	offset := 0
	for n != nil && n.left != nil {
		if index < n.left.sum.units {
			n = n.left
		} else {
			index -= n.left.sum.units
			offset += n.left.sum.runes
			n = n.right
		}
	}
	if n == nil {
		return offset
	}
	if n.sum.runes == n.sum.units {
		return offset + min(index, n.sum.runes)
	}
	for _, r := range n.text {
		count := utf16.RuneLen(r)
		if index < count {
			break
		}
		index -= count
		offset++
	}
	return offset
}
func textPrefix(n *textNode, index int) textSummary {
	if n == nil || index <= 0 {
		return textSummary{}
	}
	if index >= n.sum.bytes {
		return n.sum
	}
	if n.left == nil {
		return summarizeText(n.text[:index])
	}
	if index <= n.left.sum.bytes {
		return textPrefix(n.left, index)
	}
	return addText(n.left.sum, textPrefix(n.right, index-n.left.sum.bytes))
}
func textLineStart(n *textNode, line int) int {
	if line <= 0 {
		return 0
	}
	if line > textSum(n).lines {
		return textSum(n).bytes
	}
	offset := 0
	for n.left != nil {
		if line <= n.left.sum.lines {
			n = n.left
		} else {
			line -= n.left.sum.lines
			offset += n.left.sum.bytes
			n = n.right
		}
	}
	from := 0
	for range line {
		from += strings.IndexByte(n.text[from:], '\n') + 1
	}
	return offset + from
}
func textRead(n *textNode, start, end int, out *strings.Builder) {
	if n == nil || start >= end {
		return
	}
	if n.left == nil {
		out.WriteString(n.text[start:end])
		return
	}
	split := n.left.sum.bytes
	if start < split {
		textRead(n.left, start, min(end, split), out)
	}
	if end > split {
		textRead(n.right, max(0, start-split), end-split, out)
	}
}
