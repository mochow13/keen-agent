package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	replcommands "github.com/mochow13/keen-agent/internal/cli/repl/commands"
	repltheme "github.com/mochow13/keen-agent/internal/cli/repl/theme"
)

type suggestionMode int

const (
	commandMode suggestionMode = iota
	fileMode
)

const maxVisibleItems = 6

// SuggestionItem is a generic suggestion entry used for both slash commands and file paths.
type SuggestionItem struct {
	Name        string
	Description string
}

type SuggestionModel struct {
	visible      bool
	items        []SuggestionItem
	selected     int
	scrollOffset int
	mode         suggestionMode
}

func NewSuggestionModel() SuggestionModel {
	return SuggestionModel{}
}

func (s *SuggestionModel) Refresh(input string) {
	s.RefreshWithSkills(input, nil)
}

func (s *SuggestionModel) RefreshWithSkills(input string, skills []SuggestionItem) {
	s.mode = commandMode
	cmds := replcommands.Filter(input)
	s.items = make([]SuggestionItem, 0, len(cmds)+len(skills))
	for _, c := range cmds {
		s.items = append(s.items, SuggestionItem{Name: c.Name, Description: c.Description})
	}
	prefix := strings.ToLower(strings.TrimPrefix(input, "/"))
	for _, skill := range skills {
		name := strings.TrimPrefix(skill.Name, "/")
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			if !strings.HasPrefix(skill.Name, "/") {
				skill.Name = "/" + skill.Name
			}
			s.items = append(s.items, skill)
		}
	}
	if len(s.items) > 0 {
		s.visible = true
		s.selected = 0
		s.scrollOffset = 0
	} else {
		s.visible = false
		s.items = nil
	}
}

// RefreshFiles sets file path suggestions directly.
func (s *SuggestionModel) RefreshFiles(paths []string) {
	s.mode = fileMode
	if len(paths) == 0 {
		s.visible = false
		s.items = nil
		return
	}
	s.items = make([]SuggestionItem, len(paths))
	for i, p := range paths {
		s.items[i] = SuggestionItem{Name: p}
	}
	s.visible = true
	s.selected = 0
	s.scrollOffset = 0
}

func (s *SuggestionModel) Hide() {
	s.visible = false
	s.items = nil
}

func (s *SuggestionModel) MoveDown() {
	if len(s.items) == 0 {
		return
	}
	s.selected = (s.selected + 1) % len(s.items)
	s.adjustScroll()
}

func (s *SuggestionModel) MoveUp() {
	if len(s.items) == 0 {
		return
	}
	s.selected = (s.selected - 1 + len(s.items)) % len(s.items)
	s.adjustScroll()
}

func (s *SuggestionModel) adjustScroll() {
	if s.selected < s.scrollOffset {
		s.scrollOffset = s.selected
	} else if s.selected >= s.scrollOffset+maxVisibleItems {
		s.scrollOffset = s.selected - maxVisibleItems + 1
	}
}

func (s SuggestionModel) Current() *SuggestionItem {
	if !s.visible || len(s.items) == 0 {
		return nil
	}
	return &s.items[s.selected]
}

func (s SuggestionModel) First() *SuggestionItem {
	if len(s.items) == 0 {
		return nil
	}
	return &s.items[0]
}

func (s SuggestionModel) IsFileMode() bool {
	return s.mode == fileMode
}

func (s SuggestionModel) Height() int {
	if !s.visible {
		return 0
	}
	visible := len(s.items)
	if visible > maxVisibleItems {
		visible = maxVisibleItems
	}
	return visible + 2
}

func (s SuggestionModel) Visible() bool {
	return s.visible
}

func (s SuggestionModel) View(width int) string {
	if !s.visible {
		return ""
	}

	end := s.scrollOffset + maxVisibleItems
	if end > len(s.items) {
		end = len(s.items)
	}
	visible := s.items[s.scrollOffset:end]

	cmdColWidth := 0
	for _, item := range visible {
		if len(item.Name) > cmdColWidth {
			cmdColWidth = len(item.Name)
		}
	}
	cmdColWidth += 2

	// 2 for border + 2 for container padding
	maxDescWidth := width - cmdColWidth - 4
	// Account for description's own left padding (2)
	maxDescWidth -= 2

	var rows []string
	for i, item := range visible {
		isSelected := (i + s.scrollOffset) == s.selected

		var cmdStyle, descStyle lipgloss.Style
		if isSelected {
			cmdStyle = repltheme.SuggestionSelectedCmdStyle.Width(cmdColWidth)
			descStyle = repltheme.SuggestionSelectedDescStyle
		} else {
			cmdStyle = repltheme.SuggestionCmdStyle.Width(cmdColWidth)
			descStyle = repltheme.SuggestionDescStyle
		}

		var row string
		if item.Description != "" {
			desc := item.Description
			if maxDescWidth > 3 && len(desc) > maxDescWidth {
				desc = desc[:maxDescWidth-3] + "..."
			} else if maxDescWidth <= 3 {
				desc = ""
			}
			if desc != "" {
				row = lipgloss.JoinHorizontal(lipgloss.Left,
					cmdStyle.Render(item.Name),
					descStyle.Render(desc),
				)
			} else {
				row = cmdStyle.Render(item.Name)
			}
		} else {
			row = cmdStyle.Render(item.Name)
		}
		rows = append(rows, row)
	}

	inner := strings.Join(rows, "\n")

	hasSelection := s.selected >= 0 && len(s.items) > 0
	containerStyle := repltheme.SuggestionContainerStyle
	if hasSelection {
		containerStyle = containerStyle.BorderForeground(repltheme.PrimaryColor)
	}

	box := containerStyle.Render(inner)

	boxWidth := lipgloss.Width(box)
	if boxWidth < width {
		lines := strings.Split(box, "\n")
		var padded []string
		for _, l := range lines {
			lw := lipgloss.Width(l)
			if lw < width {
				padded = append(padded, l+strings.Repeat(" ", width-lw))
			} else {
				padded = append(padded, l)
			}
		}
		return strings.Join(padded, "\n")
	}
	return box
}
