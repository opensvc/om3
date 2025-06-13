package tree

import (
	"fmt"
	"os"
	"reflect"
	"regexp"

	"golang.org/x/term"

	"github.com/fatih/color"
)

const (
	// using unicodes
	lastNode         = "└ "
	nextNode         = "├ "
	contNode         = "│ "
	contLastNode     = "  "
	defaultSeparator = "  "
	prefixLen        = 2

	// using ascii
	//
	// lastNode         = "`- "
	// nextNode         = "|- "
	// contNode         = "|  "
	// contLastNode     = "   "
	// defaultSeparator = "   "
	// prefixLen        = 3
)

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var re = regexp.MustCompile(ansi)

func realLen(s string) int {
	return len(stripAnsi(s))
}

func stripAnsi(s string) string {
	return re.ReplaceAllString(s, "")
}

type (
	//
	// Tree exposes methods to populate and print a Tree.
	//
	// Example:
	//
	// Tree := tree.New()
	// overallNode := Tree.AddNode()
	// overallNode.AddColumn("overall")
	//
	// node := overallNode.AddNode()
	// node.AddColumn("avail")
	// node.AddColumn()
	// node.AddColumn("up", color.Green)
	// node = node.AddNode()
	// node.AddColumn("res#id")
	// node.AddColumn("....")
	// node.AddColumn("up", color.Green)
	// col := node.AddColumn("label")
	// col.AddText("warn", color.Yellow)
	// col.AddText("err", color.Red)
	//
	Tree struct {
		Separator   string
		Widths      []Width
		ForcedWidth int

		head        *Node
		totalWidth  int
		pads        []int
		columnCount int
		depth       int
	}

	// Width describes a user-defined column width preference
	Width struct {
		Min   int
		Max   int
		Exact int
	}

	// Node exposes methods to add columns.
	Node struct {
		Forest    *Tree
		Parent    *Node
		columns   []*Column
		children  []*Node
		depth     int
		cellCount int
	}

	// TextBlock is a colored and aligned phrase in a column.
	TextBlock struct {
		Text  string
		color color.Attribute
		align Alignment
	}

	// Cell is a colored and aligned cell in a column. Cells are the result
	// of phrases wrapping to respect the column width. The color and alignment
	// are inhérited from the origin TextBlock.
	Cell struct {
		Text  string
		color color.Attribute
		align Alignment
	}

	// Column exposes a method to add extra text blocks
	Column struct {
		Text  []*TextBlock
		Cells []*Cell
		Color color.Attribute
		Align Alignment
		index int
		node  *Node
	}

	// Alignment declares alignment constants as integers
	Alignment int
)

const (
	// AlignLeft is the text left alignment constant
	AlignLeft Alignment = iota
	// AlignRight is the text right alignment constant
	AlignRight
)

// New allocates a new tree and returns a reference.
func New() *Tree {
	t := &Tree{
		Separator: defaultSeparator,
		head:      &Node{},
		depth:     0,
	}
	t.head.Forest = t
	t.Widths = make([]Width, 0)
	return t
}

// Head return the tree head Node reference.
func (t *Tree) Head() *Node {
	return t.head
}

// IsEmpty returns true if the tree head node has no children.
func (t *Tree) IsEmpty() bool {
	return len(t.head.children) == 0
}

// AddNode adds and returns a new Node, child of this node.
func (t *Tree) AddNode() *Node {
	return t.head.AddNode()
}

// PlugTree add an existing tree head Node as child of the head node of the tree
func (t *Tree) PlugTree(n *Tree) {
	t.head.PlugTree(n)
}

// PlugNode add an existing Node as child of the head node of the tree
func (t *Tree) PlugNode(n *Node) {
	t.head.PlugNode(n)
}

// AddColumn adds and returns a column to the head node.
// Phrases can be added through the returned Column object.
func (t *Tree) AddColumn() *Column {
	return t.head.AddColumn()
}

// Recurse the data and return the tree buffer string.
func (t *Tree) renderRecurse(n *Node, buff string, depth int, lasts []bool) string {
	var (
		i     int
		col   *Column
		cell  *Cell
		child *Node
	)
	nChildren := len(n.children)
	lastChildIndex := nChildren - 1
	for j := 0; j < n.cellCount; j++ {
		prefix, adjust := formatPrefix(lasts, nChildren, j == 0)
		buff += prefix
		for i, col = range n.columns {
			width := t.pads[i]
			if i == 0 {
				// adjust for col0 alignment shifting due to the prefix
				width += (t.depth - depth + adjust) * prefixLen
			}
			if j >= len(col.Cells) {
				cell = &Cell{}
			} else {
				cell = col.Cells[j]
			}
			buff += col.formatCell(cell.Text, width, cell.color)
		}
		buff += "\n"
	}
	for i, child = range n.children {
		last := i == lastChildIndex
		cLasts := append(lasts, last)
		buff = t.renderRecurse(child, buff, depth+1, cLasts)
	}
	return buff
}

func (t *Tree) setTotalWidth() {
	if t.ForcedWidth > 0 {
		t.totalWidth = t.ForcedWidth
		return
	}
	columns, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		columns, _, err = term.GetSize(int(os.Stdin.Fd()))
		if err != nil {
			columns = 80
		}
	}
	t.totalWidth = columns - 4
}

// Render returns the string representation of the tree.
//
// Each node is considered tabular, with cells content aligned and wrapped.
//
// The widths parameter can used to set per-column min/max or exact widths:
//
// widths = [
//
//	(0, 10),   # col1: min 0, max 10 chars
//	None,      # col2: no constraints, auto detect
//	10         # col3: exactly 10 chars
//
// ]
func (t *Tree) Render() string {
	t.setTotalWidth()
	t.getPads()
	t.adjustPads()
	t.wrapData()
	return t.renderRecurse(t.head, "", 0, make([]bool, 0))
}

// getPads analyse data length in data columns and set the tree pads as a list
// of columns length, with no regards to terminal width constraint.
func (t *Tree) getPads() {
	t.pads = make([]int, t.columnCount)
	t.head.getPads()
}

// adjustPads distributes the termincal width amongst columns.
func (t *Tree) adjustPads() {
	var (
		i   int
		pad int
	)
	maxPrefixLen := t.depth * prefixLen
	width := 0
	for _, pad = range t.pads {
		width += pad
	}
	oversize := width - t.totalWidth
	if oversize <= 0 {
		return // no width pressure, unchanged pads
	}
	avgColumnWidth := t.totalWidth / t.columnCount
	usableWidth := t.totalWidth - maxPrefixLen - t.pads[0]
	oversizedColumnCount := 0
	for _, pad = range t.pads[1:] {
		if pad > avgColumnWidth {
			oversizedColumnCount++
		} else {
			usableWidth -= pad
		}
	}
	maxWidth := usableWidth / oversizedColumnCount
	for i, pad = range t.pads[1:] {
		if pad > avgColumnWidth {
			t.pads[i+1] = maxWidth
		}
	}
}

// wrapData transforms column textblocks into cells
func (t *Tree) wrapData() {
	t.head.wrapData()
}

// PlugTree add an existing tree head Node as child this node.
func (t *Node) PlugTree(n *Tree) {
	t.PlugNode(n.Head())
}

// PlugNode adds and existing Node as child of this node.
func (t *Node) PlugNode(child *Node) {
	assignNode(t, child)
	t.children = append(t.children, child)
}

// AddNode adds and returns a new Node, child of this node.
func (t *Node) AddNode() *Node {
	newNode := &Node{
		Forest: t.Forest,
		Parent: t,
		depth:  t.depth + 1,
	}
	t.children = append(t.children, newNode)
	if newNode.depth > t.Forest.depth {
		t.Forest.depth = newNode.depth
	}
	return newNode
}

// getPads recursively analyses nodes
func (t *Node) getPads() {
	for idx, col := range t.columns {
		width := t.Forest.Widths[idx]
		if width.Exact > 0 {
			t.Forest.pads[idx] = width.Exact
			continue
		}
		for _, fragment := range col.Text {
			fragmentWidth := realLen(fragment.Text) + len(t.Forest.Separator)
			if fragmentWidth > t.Forest.pads[idx] {
				if width.Min > 0 && t.Forest.pads[idx] < width.Min {
					t.Forest.pads[idx] = width.Min
				} else if width.Max > 0 && t.Forest.pads[idx] > width.Max {
					t.Forest.pads[idx] = width.Max
				} else {
					t.Forest.pads[idx] = fragmentWidth
				}
			}
		}
	}
	for _, child := range t.children {
		child.getPads()
	}
}

func (t *Node) wrapData() {
	var (
		i   int
		col *Column
	)
	for i, col = range t.columns {
		for _, fragment := range col.Text {
			for _, line := range col.wrappedLines(fragment.Text, t.Forest.pads[i]) {
				cell := &Cell{
					Text:  line,
					color: fragment.color,
					align: col.Align,
				}
				col.Cells = append(col.Cells, cell)
			}
		}
		colLineCount := len(col.Cells)
		if colLineCount > t.cellCount {
			t.cellCount = colLineCount
		}
	}
	for _, child := range t.children {
		child.wrapData()
	}
}

func (t *Node) String() string {
	s := ""
	s += fmt.Sprintf("<node Forest:%p >", t.Forest)
	return s
}

// IsEmpty returns true if the node has one or more columns
func (t *Node) IsEmpty() bool {
	return len(t.columns) == 0
}

// AddColumn adds and returns a column to the node.
// Phrases can be added through the returned Column object.
func (t *Node) AddColumn() *Column {
	c := &Column{}
	columnCount := len(t.columns)
	c.node = t
	c.index = columnCount
	t.columns = append(t.columns, c)
	columnCount++
	if columnCount > t.Forest.columnCount {
		t.Forest.columnCount = columnCount
		t.Forest.Widths = append(t.Forest.Widths, Width{})
	}
	return c
}

// Load loads data in the node
//
// Example dataset:
//
//	{
//	    "data": [
//	        {
//	            "text": "node1",
//	            "color": color.BOLD
//	        }
//	    ],
//	    "children": [
//	        {
//	            "data": [
//	                {
//	                    "text": "node2"
//	                },
//	                {
//	                    "text": "foo",
//	                    "color": color.RED
//	                }
//	            ],
//	            "children": [
//	            ]
//	        }
//	    ]
//	}
//
// would be rendered as:
//
// node1
// `- node2 foo
func (t *Node) Load(data interface{}, title string) {
	head := t
	if title == "" {
		head.AddColumn().AddText(title).SetColor(color.Bold)
	}
	loadRecurse(head, data)
}

// formatCell returns the table cell, appending the separator, coloring the
// text and applying the padding for alignment.
func (c *Column) formatCell(text string, width int, textColor color.Attribute) string {
	var f string
	width += len(text) - realLen(text)
	switch c.Align {
	case AlignRight:
		f = fmt.Sprintf("%%%ds", width)
	case AlignLeft:
		f = fmt.Sprintf("%%-%ds", width)
	}
	buff := fmt.Sprintf(f, text)
	return color.New(textColor).Sprint(buff)
}

// wrappedLines return lines split by the text wrapper wrapping at <width>.
func (c *Column) wrappedLines(text string, width int) []string {
	lines := make([]string, 0)
	if width == 0 {
		return lines
	}
	offset := 0
	remain := realLen(text)
	for remain > width {
		lines = append(lines, text[:width])
		text = text[width:]
		remain -= width
	}
	lines = append(lines, text[offset:])
	return lines
}

// AddText adds a colored and aligned phrase to this column.
func (c *Column) AddText(text string) *TextBlock {
	t := &TextBlock{
		Text: text,
	}
	c.Text = append(c.Text, t)
	return t
}

// SetColor sets the text block color and returns the textBlock ref
// so the caller can chain AddText("").SetColor().SetAlign()
func (t *TextBlock) SetColor(textColor color.Attribute) *TextBlock {
	t.color = textColor
	return t
}

// SetAlign sets the text block alignment and returns the textBlock ref
// so the caller can chain AddText("").SetColor().SetAlign()
func (t *TextBlock) SetAlign(align Alignment) *TextBlock {
	t.align = align
	return t
}

// assignNode set the Forest and Parent fields on child and recursively all descendent nodes of child.
func assignNode(parent, child *Node) {
	if child == nil {
		return
	}
	child.Forest = parent.Forest
	child.Parent = parent
	child.depth = parent.depth + 1
	if child.depth > parent.Forest.depth {
		parent.Forest.depth = child.depth
	}
	columnCount := len(child.columns)
	if columnCount > child.Forest.columnCount {
		child.Forest.columnCount = columnCount
		child.Forest.Widths = make([]Width, columnCount)
	}
	for _, grandchild := range child.children {
		assignNode(child, grandchild)
	}
}

// formatPrefix returns the tree markers as a string for a line.
func formatPrefix(lasts []bool, nChildren int, firstLine bool) (string, int) {
	var adjust int
	if len(lasts) == 0 {
		return "", adjust
	}
	buff := ""
	if firstLine {
		// new node
		for _, last := range lasts[:len(lasts)-1] {
			if last {
				buff += contLastNode
			} else {
				buff += contNode
			}
		}
		if lasts[len(lasts)-1] {
			buff += lastNode
		} else {
			buff += nextNode
		}
	} else {
		// node continuation due to wrapping
		for _, last := range lasts[:len(lasts)-1] {
			if last {
				buff += contLastNode
			} else {
				buff += contNode
			}
		}
		last := lasts[len(lasts)-1]
		if nChildren > 0 || !last {
			if last {
				buff += defaultSeparator
				adjust = -1
			}
			buff += contNode
		} else {
			buff += contLastNode
		}
	}
	return buff, adjust
}

// loadRecurse switches between data loaders
func loadRecurse(head *Node, data interface{}) {
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Array:
		loadList(head, data.([]interface{}))
	case reflect.Map:
		loadMap(head, data.(map[string]interface{}))
	default:
		head.AddColumn().AddText(fmt.Sprint(data))
	}
}

func loadList(head *Node, _data []interface{}) {
	for idx, val := range _data {
		leaf := head.AddNode()
		leaf.AddColumn().AddText(fmt.Sprintf("[%d]", idx))
		loadRecurse(leaf, val)
	}
}

// Load data structured as dict in the node.
func loadMap(head *Node, _data map[string]interface{}) {
	for key, val := range _data {
		leaf := head.AddNode()
		leaf.AddColumn().AddText(key).SetColor(color.FgHiBlack)
		loadRecurse(leaf, val)
	}
}
