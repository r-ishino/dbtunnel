package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// ErrCancelled は選択がキャンセルされた場合のエラー
	ErrCancelled = errors.New("選択がキャンセルされました")

	// スタイルを事前定義（パフォーマンス向上）
	docStyle = lipgloss.NewStyle().Margin(1, 2)

	titleStyle = lipgloss.NewStyle().
			MarginLeft(2).
			Foreground(lipgloss.Color("170")).
			Bold(true)

	normalItemStyle = lipgloss.NewStyle().PaddingLeft(2)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170"))

	helpStyle = lipgloss.NewStyle().
			MarginTop(1).
			MarginLeft(2).
			Foreground(lipgloss.Color("241"))
)

// selectorModel はBubbleteaのモデル
type selectorModel struct {
	title    string
	options  []string
	cursor   int
	choice   string
	quitting bool
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// スペースキーはKeyTypeで判定（IME問題対策）
		if msg.Type == tea.KeySpace {
			m.choice = m.options[m.cursor]
			return m, tea.Quit
		}

		switch msg.String() {
		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}

		case "　", "enter": // 全角スペースまたはEnterで確定
			m.choice = m.options[m.cursor]
			return m, tea.Quit

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// 数字キーで直接選択（IME問題の代替手段）
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(m.options) {
				m.choice = m.options[idx]
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m selectorModel) View() string {
	if m.choice != "" || m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, option := range m.options {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		line := fmt.Sprintf("%s%d. %s", cursor, i+1, option)

		if m.cursor == i {
			b.WriteString(selectedItemStyle.Render(line))
		} else {
			b.WriteString(normalItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("↑/↓: 移動  Space/Enter/数字: 選択  Esc: キャンセル"))

	return docStyle.Render(b.String())
}

// Select は選択肢を表示してユーザーの選択を返す
func Select(title string, options []string) (string, error) {
	if len(options) == 0 {
		return "", errors.New("選択肢がありません")
	}

	m := selectorModel{
		title:   title,
		options: options,
		cursor:  0,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("UIの実行に失敗しました: %w", err)
	}

	fm := finalModel.(selectorModel)
	if fm.quitting {
		return "", ErrCancelled
	}

	return fm.choice, nil
}

// multiSelectModel は複数選択用のBubbleteaモデル
type multiSelectModel struct {
	title    string
	options  []string
	cursor   int
	selected map[int]bool
	quitting bool
}

func (m multiSelectModel) Init() tea.Cmd {
	return nil
}

func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// スペースキーはKeyTypeで判定（IME問題対策）
		if msg.Type == tea.KeySpace {
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}
			return m, nil
		}

		switch msg.String() {
		case "esc", "ctrl+c":
			m.quitting = true
			m.selected = nil
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}

		case "　", "x", "tab": // 全角スペース/x/tabで選択をトグル
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}

		case "a":
			// 全選択/全解除
			if len(m.selected) == len(m.options) {
				m.selected = make(map[int]bool)
			} else {
				for i := range m.options {
					m.selected[i] = true
				}
			}

		case "enter":
			// 1件も選択されていない場合は無視
			if len(m.selected) > 0 {
				return m, tea.Quit
			}

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// 数字キーで直接トグル（IME問題の代替手段）
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(m.options) {
				if m.selected[idx] {
					delete(m.selected, idx)
				} else {
					m.selected[idx] = true
				}
			}
		}
	}

	return m, nil
}

func (m multiSelectModel) View() string {
	if m.quitting && m.selected == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, option := range m.options {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		checked := "[ ]"
		if m.selected[i] {
			checked = "[x]"
		}

		line := fmt.Sprintf("%s%d. %s %s", cursor, i+1, checked, option)
		if m.cursor == i {
			b.WriteString(selectedItemStyle.Render(line))
		} else {
			b.WriteString(normalItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("x/スペース/数字: 選択  a: 全選択  Enter: 確定  Esc: キャンセル"))

	return docStyle.Render(b.String())
}

// SelectMultiple は複数選択肢を表示してユーザーの選択を返す
func SelectMultiple(title string, options []string) ([]string, error) {
	if len(options) == 0 {
		return nil, errors.New("選択肢がありません")
	}

	m := multiSelectModel{
		title:    title,
		options:  options,
		cursor:   0,
		selected: make(map[int]bool),
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("UIの実行に失敗しました: %w", err)
	}

	fm := finalModel.(multiSelectModel)
	if fm.quitting && fm.selected == nil {
		return nil, ErrCancelled
	}

	// 選択された項目を順序通りに返す
	var result []string
	for i, opt := range options {
		if fm.selected[i] {
			result = append(result, opt)
		}
	}

	return result, nil
}
