package history

import (
	"bufio"
	"os"
	"strings"
)

const MaxHistorySize = 1000

type InputHistory struct {
	entries  []string
	idx      int
	draft    string
	filePath string
}

func (h *InputHistory) Push(entry string) {
	if strings.TrimSpace(entry) == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == entry {
		return
	}
	h.entries = append(h.entries, entry)
	if len(h.entries) > MaxHistorySize {
		h.entries = h.entries[len(h.entries)-MaxHistorySize:]
	}
	h.idx = -1
	h.draft = ""
	_ = h.AppendToFile(entry)
}

func (h *InputHistory) NavigateUp(current string) (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	if h.idx == -1 {
		h.draft = current
		h.idx = len(h.entries) - 1
		return h.entries[h.idx], true
	}
	if h.idx == 0 {
		return "", false
	}
	h.idx--
	return h.entries[h.idx], true
}

func (h *InputHistory) NavigateDown() (string, bool) {
	if len(h.entries) == 0 || h.idx == -1 {
		return "", false
	}
	if h.idx == len(h.entries)-1 {
		h.idx = -1
		return h.draft, true
	}
	h.idx++
	return h.entries[h.idx], true
}

func (h *InputHistory) Reset() {
	h.idx = -1
	h.draft = ""
}

func (h *InputHistory) IsNavigating() bool {
	return h.idx != -1
}

func (h *InputHistory) LoadFromFile(path string) error {
	h.filePath = path
	h.idx = -1
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	var entries []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		entry := strings.ReplaceAll(line, `\n`, "\n")
		if len(entries) > 0 && entries[len(entries)-1] == entry {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if len(entries) > MaxHistorySize {
		entries = entries[len(entries)-MaxHistorySize:]
	}
	h.entries = entries
	return nil
}

func (h *InputHistory) Flush() error {
	if h.filePath == "" {
		return nil
	}
	f, err := os.OpenFile(h.filePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, entry := range h.entries {
		escaped := strings.ReplaceAll(entry, "\n", `\n`)
		if _, err := w.WriteString(escaped + "\n"); err != nil {
			return err
		}
	}
	return w.Flush()
}

func (h *InputHistory) AppendToFile(entry string) error {
	if h.filePath == "" {
		return nil
	}
	escaped := strings.ReplaceAll(entry, "\n", `\n`)
	f, err := os.OpenFile(h.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(escaped + "\n")
	return err
}
