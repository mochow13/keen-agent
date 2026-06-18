package repl

import (
	"image"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/displaywidth"
	"github.com/clipperhouse/uax29/v2/words"
)

const (
	selectionClickThreshold = 400 * time.Millisecond
	selectionClickTolerance = 2
	inputPromptWidth        = 3
)

type selectionPoint struct {
	line int
	col  int
}

type viewportSelection struct {
	lines []string

	mouseDown bool
	anchor    selectionPoint
	cursor    selectionPoint

	lastClickTime time.Time
	lastClickX    int
	lastClickY    int
	clickCount    int
}

func (s *viewportSelection) setContent(content string) {
	if content == "" {
		s.lines = nil
		return
	}
	s.lines = strings.Split(content, "\n")
}

func (s *viewportSelection) start(localX, localY, yOffset int) {
	p := selectionPoint{line: clampInt(yOffset+localY, 0, max(0, len(s.lines)-1)), col: max(localX, 0)}
	s.mouseDown = true
	s.anchor = p
	s.cursor = p
}

func (s *viewportSelection) drag(localX, localY, yOffset int) bool {
	if !s.mouseDown {
		return false
	}
	s.cursor = selectionPoint{line: clampInt(yOffset+localY, 0, max(0, len(s.lines)-1)), col: max(localX, 0)}
	return true
}

func (s *viewportSelection) release() bool {
	if !s.mouseDown {
		return false
	}
	s.mouseDown = false
	return true
}

func (s *viewportSelection) clear() {
	s.mouseDown = false
	s.anchor = selectionPoint{}
	s.cursor = selectionPoint{}
	s.clickCount = 0
}

func (s viewportSelection) hasSelection() bool {
	return s.anchor.line != s.cursor.line || s.anchor.col != s.cursor.col
}

func (s *viewportSelection) registerClick(localX, localY int) int {
	now := time.Now()
	if now.Sub(s.lastClickTime) <= selectionClickThreshold &&
		absInt(localX-s.lastClickX) <= selectionClickTolerance &&
		absInt(localY-s.lastClickY) <= selectionClickTolerance {
		s.clickCount++
	} else {
		s.clickCount = 1
	}
	s.lastClickTime = now
	s.lastClickX = localX
	s.lastClickY = localY
	return s.clickCount
}

func (s *viewportSelection) selectWord(localX, localY, yOffset int) bool {
	lineIndex := clampInt(yOffset+localY, 0, max(0, len(s.lines)-1))
	if lineIndex < 0 {
		return false
	}
	line := ansi.Strip(s.lines[lineIndex])
	startCol, endCol := findSelectionWordBoundaries(line, max(localX, 0))
	if startCol == endCol {
		return false
	}
	s.mouseDown = true
	s.anchor = selectionPoint{line: lineIndex, col: startCol}
	s.cursor = selectionPoint{line: lineIndex, col: endCol}
	return true
}

func (s *viewportSelection) selectLine(localY, yOffset int) bool {
	lineIndex := clampInt(yOffset+localY, 0, max(0, len(s.lines)-1))
	if lineIndex < 0 {
		return false
	}
	lineWidth := ansi.StringWidth(s.lines[lineIndex])
	if lineWidth == 0 {
		return false
	}
	s.mouseDown = true
	s.anchor = selectionPoint{line: lineIndex, col: 0}
	s.cursor = selectionPoint{line: lineIndex, col: lineWidth}
	return true
}

func (s viewportSelection) selectedText() string {
	if !s.hasSelection() {
		return ""
	}
	start, end := s.normalizedRange()
	if len(s.lines) == 0 {
		return ""
	}
	end.line = min(end.line, len(s.lines)-1)
	start.line = min(start.line, len(s.lines)-1)
	if start.line > end.line {
		return ""
	}

	parts := make([]string, 0, end.line-start.line+1)
	for lineIndex := start.line; lineIndex <= end.line; lineIndex++ {
		line := ansi.Strip(s.lines[lineIndex])
		lineWidth := ansi.StringWidth(line)
		colStart := 0
		if lineIndex == start.line {
			colStart = clampInt(start.col, 0, lineWidth)
		}
		colEnd := lineWidth
		if lineIndex == end.line {
			colEnd = clampInt(end.col, 0, lineWidth)
		}
		if colEnd < colStart {
			colStart, colEnd = colEnd, colStart
		}
		parts = append(parts, ansi.Cut(line, colStart, colEnd))
	}
	return strings.TrimRight(strings.Join(parts, "\n"), "\n")
}

func (s viewportSelection) render(view string, width, height, yOffset int) string {
	return renderSelection(view, width, height, yOffset, 0, s.anchor, s.cursor)
}

func renderSelection(view string, width, height, yOffset, colOffset int, anchor, cursor selectionPoint) string {
	if (anchor.line == cursor.line && anchor.col == cursor.col) || width <= 0 || height <= 0 || view == "" {
		return view
	}

	start, end := anchor, cursor
	if end.line < start.line || (end.line == start.line && end.col < start.col) {
		start, end = end, start
	}

	if end.line < yOffset || start.line >= yOffset+height {
		return view
	}

	startLine := max(0, start.line-yOffset)
	endLine := min(height-1, end.line-yOffset)
	startCol := 0
	if start.line >= yOffset {
		startCol = start.col + colOffset
	}
	endCol := width
	if end.line < yOffset+height {
		endCol = end.col + colOffset
	}

	buf := uv.NewScreenBuffer(width, height)
	uv.NewStyledString(view).Draw(&buf, image.Rect(0, 0, width, height))

	for y := startLine; y <= endLine; y++ {
		line := buf.Line(y)
		if line == nil {
			continue
		}
		colStart := 0
		if y == startLine {
			colStart = clampInt(startCol, 0, width)
		}
		colEnd := width
		if y == endLine {
			colEnd = clampInt(endCol, 0, width)
		}

		lastContentX := -1
		for x := colStart; x < colEnd; x++ {
			cell := line.At(x)
			if cell == nil || cell.IsZero() {
				continue
			}
			if cell.Content != "" && cell.Content != " " {
				lastContentX = x
			}
		}
		if lastContentX >= 0 {
			colEnd = min(colEnd, lastContentX+1)
		} else if colEnd == width {
			colEnd = colStart
		}

		for x := colStart; x < colEnd; x++ {
			cell := line.At(x)
			if cell == nil || cell.IsZero() {
				continue
			}
			cell.Style.Attrs |= uv.AttrReverse
		}
	}

	return buf.Render()
}

func (s viewportSelection) normalizedRange() (selectionPoint, selectionPoint) {
	start, end := s.anchor, s.cursor
	if end.line < start.line || (end.line == start.line && end.col < start.col) {
		start, end = end, start
	}
	return start, end
}

func findSelectionWordBoundaries(line string, col int) (startCol, endCol int) {
	if line == "" || col < 0 {
		return 0, 0
	}

	lineCol := 0
	lastCol := 0
	iter := words.FromString(line)
	for iter.Next() {
		token := iter.Value()
		tokenWidth := displaywidth.String(token)
		tokenStart := lineCol
		tokenEnd := lineCol + tokenWidth
		lineCol += tokenWidth

		if col < tokenStart {
			return lastCol, lastCol
		}
		lastCol = tokenEnd

		if col >= tokenStart && col < tokenEnd {
			if strings.TrimSpace(token) == "" {
				return col, col
			}
			return tokenStart, tokenEnd
		}
	}

	return col, col
}

func copySelectedTextCmd(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	return tea.Sequence(
		tea.SetClipboard(text),
		func() tea.Msg {
			_ = clipboard.WriteAll(text)
			return nil
		},
	)
}

func clearCopyNotificationCmd(expiresAt time.Time) tea.Cmd {
	return tea.Tick(time.Until(expiresAt), func(time.Time) tea.Msg {
		return copyNotificationExpiredMsg{expiresAt: expiresAt.UnixNano()}
	})
}

func (m *replModel) copySelectedTextCmd(text string) tea.Cmd {
	cmd := copySelectedTextCmd(text)
	if cmd == nil {
		return nil
	}
	expiresAt := time.Now().Add(copyNotificationTimeout)
	m.copyNotification = copyNotificationMessage
	m.copyNotificationExpiresAt = expiresAt
	return tea.Batch(cmd, clearCopyNotificationCmd(expiresAt))
}

func (m *replModel) openURLAtMouse(x, y int) tea.Cmd {
	lineIndex := m.viewport.YOffset() + y
	if lineIndex < 0 || lineIndex >= len(m.selection.lines) {
		return nil
	}
	url := urlAtDisplayColumn(m.selection.lines[lineIndex], max(x, 0))
	if url == "" {
		return nil
	}
	return func() tea.Msg {
		_ = openURL(url)
		return nil
	}
}

func (m *replModel) handleSelectionMouseDown(msg tea.MouseClickMsg) (bool, tea.Cmd) {
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft {
		return false, nil
	}
	if !m.mouseInViewport(mouse.X, mouse.Y) {
		if m.selection.hasSelection() {
			m.selection.clear()
		}
		return false, nil
	}

	if mouse.Mod&(tea.ModSuper|tea.ModAlt|tea.ModCtrl) != 0 {
		if cmd := m.openURLAtMouse(mouse.X, mouse.Y); cmd != nil {
			return true, cmd
		}
	}

	m.blurInput()
	clickCount := m.selection.registerClick(mouse.X, mouse.Y)
	switch clickCount {
	case 2:
		if !m.selection.selectWord(mouse.X, mouse.Y, m.viewport.YOffset()) {
			m.selection.start(mouse.X, mouse.Y, m.viewport.YOffset())
		}
	case 3:
		if !m.selection.selectLine(mouse.Y, m.viewport.YOffset()) {
			m.selection.start(mouse.X, mouse.Y, m.viewport.YOffset())
		}
		m.selection.clickCount = 0
	default:
		m.selection.start(mouse.X, mouse.Y, m.viewport.YOffset())
	}
	return true, nil
}

func (m *replModel) handleSelectionMouseDrag(msg tea.MouseMotionMsg) bool {
	if !m.selection.mouseDown {
		return false
	}
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft && mouse.Button != tea.MouseNone {
		return false
	}

	localY := mouse.Y
	if localY < 0 {
		m.viewport.ScrollUp(1)
		localY = 0
	} else if localY >= m.viewport.Height() {
		m.viewport.ScrollDown(1)
		localY = max(0, m.viewport.Height()-1)
	}
	localX := clampInt(mouse.X, 0, m.viewport.Width())
	m.selection.drag(localX, localY, m.viewport.YOffset())
	m.userScrolled = !m.viewport.AtBottom()
	return true
}

func (m *replModel) handleSelectionMouseUp() (bool, tea.Cmd) {
	if !m.selection.release() {
		return false, nil
	}
	return true, m.copySelectedTextCmd(m.selection.selectedText())
}

func (m *replModel) handleMouseDown(msg tea.MouseClickMsg) (bool, tea.Cmd) {
	if handled, cmd := m.handleInputSelectionMouseDown(msg); handled {
		return true, cmd
	}
	return m.handleSelectionMouseDown(msg)
}

func (m *replModel) handleMouseDrag(msg tea.MouseMotionMsg) bool {
	if m.inputSelection.mouseDown {
		return m.handleInputSelectionMouseDrag(msg)
	}
	return m.handleSelectionMouseDrag(msg)
}

func (m *replModel) handleMouseUp() (bool, tea.Cmd) {
	if m.inputSelection.mouseDown {
		return m.handleInputSelectionMouseUp()
	}
	return m.handleSelectionMouseUp()
}

func (m replModel) mouseInViewport(x, y int) bool {
	return x >= 0 && y >= 0 && x <= m.viewport.Width() && y < m.viewport.Height()
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clampInt(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
