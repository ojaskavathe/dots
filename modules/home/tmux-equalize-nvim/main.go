package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/neovim/go-client/nvim"
)

var tmuxPath = "tmux"

type node struct {
	kind     byte
	width    int
	height   int
	left     int
	top      int
	pane     string
	children []*node
}

type parser struct {
	text string
	i    int
}

func (p *parser) number() (int, error) {
	start := p.i
	for p.i < len(p.text) && p.text[p.i] >= '0' && p.text[p.i] <= '9' {
		p.i++
	}
	if start == p.i {
		return 0, fmt.Errorf("expected number at %d", p.i)
	}
	return strconv.Atoi(p.text[start:p.i])
}

func (p *parser) char(expected byte) error {
	if p.i >= len(p.text) || p.text[p.i] != expected {
		return fmt.Errorf("expected %q at %d", expected, p.i)
	}
	p.i++
	return nil
}

func (p *parser) node() (*node, error) {
	width, err := p.number()
	if err != nil {
		return nil, err
	}
	if err := p.char('x'); err != nil {
		return nil, err
	}
	height, err := p.number()
	if err != nil {
		return nil, err
	}
	if err := p.char(','); err != nil {
		return nil, err
	}
	left, err := p.number()
	if err != nil {
		return nil, err
	}
	if err := p.char(','); err != nil {
		return nil, err
	}
	top, err := p.number()
	if err != nil {
		return nil, err
	}
	if p.i >= len(p.text) {
		return nil, errors.New("unexpected end of layout")
	}

	n := &node{
		width:  width,
		height: height,
		left:   left,
		top:    top,
	}

	next := p.text[p.i]
	if next == ',' {
		p.i++
		pane, err := p.number()
		if err != nil {
			return nil, err
		}
		n.kind = 'l'
		n.pane = strconv.Itoa(pane)
		return n, nil
	}

	if next != '{' && next != '[' {
		return nil, fmt.Errorf("expected child list at %d", p.i)
	}
	p.i++
	n.kind = next

	close := byte(']')
	if next == '{' {
		close = '}'
	}

	for {
		child, err := p.node()
		if err != nil {
			return nil, err
		}
		n.children = append(n.children, child)

		if p.i >= len(p.text) {
			return nil, errors.New("unterminated child list")
		}
		if p.text[p.i] == close {
			p.i++
			break
		}
		if err := p.char(','); err != nil {
			return nil, err
		}
	}

	return n, nil
}

func render(n *node) string {
	prefix := fmt.Sprintf("%dx%d,%d,%d", n.width, n.height, n.left, n.top)
	if n.kind == 'l' {
		return prefix + "," + n.pane
	}

	var b strings.Builder
	b.WriteString(prefix)
	b.WriteByte(n.kind)
	for i, child := range n.children {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(render(child))
	}
	if n.kind == '{' {
		b.WriteByte('}')
	} else {
		b.WriteByte(']')
	}
	return b.String()
}

func checksum(layout string) string {
	var sum uint16
	for _, r := range layout {
		sum = (sum >> 1) + ((sum & 1) << 15)
		sum += uint16(r)
	}
	return fmt.Sprintf("%04x", sum)
}

type paneInfo struct {
	command string
	server  string
}

func tmux(args ...string) (string, error) {
	out, err := exec.Command(tmuxPath, args...).Output()
	return strings.TrimRight(string(out), "\n"), err
}

func tmuxOK(args ...string) bool {
	cmd := exec.Command(tmuxPath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func tmuxAxis(kind byte) string {
	if kind == '{' {
		return "x"
	}
	return "y"
}

func paneAxisCount(layout any, axis string) int {
	items, ok := layout.([]any)
	if !ok || len(items) < 2 {
		return 1
	}

	kind, _ := items[0].(string)
	if kind == "leaf" {
		return 1
	}

	children, ok := items[1].([]any)
	if !ok || len(children) == 0 {
		return 1
	}

	counts := make([]int, 0, len(children))
	for _, child := range children {
		counts = append(counts, paneAxisCount(child, axis))
	}

	layoutAxis := "y"
	if kind == "row" {
		layoutAxis = "x"
	}
	if layoutAxis == axis {
		total := 0
		for _, count := range counts {
			total += count
		}
		return total
	}

	maxCount := 1
	for _, count := range counts {
		maxCount = max(maxCount, count)
	}
	return maxCount
}

func visualWeight(n *node, axis string, counts map[string]map[string]int) int {
	if n.kind == 'l' {
		if paneCounts, ok := counts[n.pane]; ok {
			if count, ok := paneCounts[axis]; ok && count > 0 {
				return count
			}
		}
		return 1
	}

	weights := make([]int, 0, len(n.children))
	for _, child := range n.children {
		weights = append(weights, visualWeight(child, axis, counts))
	}

	if tmuxAxis(n.kind) == axis {
		total := 0
		for _, weight := range weights {
			total += weight
		}
		return max(total, 1)
	}

	maxWeight := 1
	for _, weight := range weights {
		maxWeight = max(maxWeight, weight)
	}
	return maxWeight
}

func allocate(total int, weights []int) []int {
	if len(weights) == 0 {
		return nil
	}

	total = max(total, len(weights))
	weightSum := 0
	for _, weight := range weights {
		weightSum += weight
	}
	if weightSum == 0 {
		weightSum = len(weights)
	}

	raw := make([]float64, len(weights))
	sizes := make([]int, len(weights))
	for i, weight := range weights {
		raw[i] = float64(total) * float64(weight) / float64(weightSum)
		sizes[i] = max(1, int(math.Floor(raw[i])))
	}

	for sumInts(sizes) > total {
		index := largestIndex(sizes)
		if sizes[index] == 1 {
			break
		}
		sizes[index]--
	}

	for remaining := total - sumInts(sizes); remaining > 0; remaining-- {
		index := largestRemainderIndex(raw, sizes)
		sizes[index]++
		raw[index] = math.Floor(raw[index])
	}

	return sizes
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func largestIndex(values []int) int {
	index := 0
	for i := range values {
		if values[i] > values[index] {
			index = i
		}
	}
	return index
}

func largestRemainderIndex(raw []float64, sizes []int) int {
	index := 0
	best := raw[0] - math.Floor(raw[0])
	for i := range raw {
		remainder := raw[i] - math.Floor(raw[i])
		if remainder > best || (remainder == best && sizes[i] < sizes[index]) {
			best = remainder
			index = i
		}
	}
	return index
}

func assign(n *node, left, top, width, height int, counts map[string]map[string]int) {
	n.left = left
	n.top = top
	n.width = width
	n.height = height

	if n.kind == 'l' {
		return
	}

	separators := len(n.children) - 1
	if n.kind == '{' {
		weights := make([]int, 0, len(n.children))
		for _, child := range n.children {
			weights = append(weights, visualWeight(child, "x", counts))
		}
		widths := allocate(width-separators, weights)
		childLeft := left
		for i, child := range n.children {
			assign(child, childLeft, top, widths[i], height, counts)
			childLeft += widths[i] + 1
		}
		return
	}

	weights := make([]int, 0, len(n.children))
	for _, child := range n.children {
		weights = append(weights, visualWeight(child, "y", counts))
	}
	heights := allocate(height-separators, weights)
	childTop := top
	for i, child := range n.children {
		assign(child, left, childTop, width, heights[i], counts)
		childTop += heights[i] + 1
	}
}

func isNvim(command string) bool {
	return command == "nvim" || command == "vim" || command == "view" || strings.HasPrefix(command, "nvim-")
}

func currentPanes(window string) map[string]paneInfo {
	lines, err := tmux("list-panes", "-t", window, "-F", "#{pane_id}\t#{pane_dead}\t#{pane_current_command}\t#{@nvim_server}")
	if err != nil {
		return nil
	}

	panes := map[string]paneInfo{}
	for _, line := range strings.Split(lines, "\n") {
		parts := strings.SplitN(line, "\t", 4)
		for len(parts) < 4 {
			parts = append(parts, "")
		}
		if parts[1] != "0" {
			continue
		}
		panes[strings.TrimPrefix(parts[0], "%")] = paneInfo{
			command: parts[2],
			server:  parts[3],
		}
	}
	return panes
}

func dialNvim(path string) (*nvim.Nvim, error) {
	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	return nvim.Dial(path, nvim.DialNetDial(dialer.DialContext))
}

func nvimLayoutCounts(panes map[string]paneInfo) (map[string]map[string]int, map[string]*nvim.Nvim) {
	counts := map[string]map[string]int{}
	clients := map[string]*nvim.Nvim{}

	for paneID, pane := range panes {
		if pane.server == "" || !isNvim(pane.command) {
			continue
		}
		if _, err := os.Stat(pane.server); err != nil {
			tmuxOK("set-option", "-pt", "%"+paneID, "-u", "@nvim_server")
			continue
		}

		client, err := dialNvim(pane.server)
		if err != nil {
			tmuxOK("set-option", "-pt", "%"+paneID, "-u", "@nvim_server")
			continue
		}

		var layout any
		if err := client.Eval("winlayout()", &layout); err != nil {
			_ = client.Close()
			tmuxOK("set-option", "-pt", "%"+paneID, "-u", "@nvim_server")
			continue
		}

		counts[paneID] = map[string]int{
			"x": paneAxisCount(layout, "x"),
			"y": paneAxisCount(layout, "y"),
		}
		clients[paneID] = client
	}

	return counts, clients
}

func run() error {
	currentPane, err := tmux("display-message", "-p", "#{pane_id}")
	if err != nil {
		return err
	}
	currentWindow, err := tmux("display-message", "-p", "#{window_id}")
	if err != nil {
		return err
	}

	layout, err := tmux("display-message", "-pt", currentWindow, "-p", "#{window_layout}")
	if err != nil {
		return err
	}
	_, body, ok := strings.Cut(layout, ",")
	if !ok {
		return fmt.Errorf("invalid tmux layout: %s", layout)
	}

	root, err := (&parser{text: body}).node()
	if err != nil {
		tmuxOK("select-layout", "-t", currentWindow, "-E")
		return err
	}

	counts, clients := nvimLayoutCounts(currentPanes(currentWindow))
	defer func() {
		for _, client := range clients {
			_ = client.Close()
		}
	}()

	assign(root, root.left, root.top, root.width, root.height, counts)
	body = render(root)
	if !tmuxOK("select-layout", "-t", currentWindow, checksum(body)+","+body) {
		tmuxOK("select-layout", "-t", currentWindow, "-E")
	}

	for paneID, client := range clients {
		if err := client.Command("wincmd ="); err != nil {
			tmuxOK("set-option", "-pt", "%"+paneID, "-u", "@nvim_server")
		}
	}

	tmuxOK("select-pane", "-t", currentPane)
	return nil
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}
