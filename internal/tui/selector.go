package tui

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// ErrCancelled は選択がキャンセルされた場合のエラー
	ErrCancelled = errors.New("選択がキャンセルされました")

	docStyle = lipgloss.NewStyle().Margin(1, 2)
)

// item はリスト項目を表す
type item struct {
	title string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.title }

// itemDelegate はリスト項目の表示をカスタマイズする
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i.title)

	fn := lipgloss.NewStyle().PaddingLeft(2).Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170")).
				Render("> " + strings.Join(s, " "))
		}
	}

	_, _ = fmt.Fprint(w, fn(str))
}

// selectorModel はBubbleteaのモデル
type selectorModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.title
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectorModel) View() string {
	if m.choice != "" {
		return ""
	}
	if m.quitting {
		return ""
	}
	return docStyle.Render(m.list.View())
}

// Select は選択肢を表示してユーザーの選択を返す
func Select(title string, options []string) (string, error) {
	if len(options) == 0 {
		return "", errors.New("選択肢がありません")
	}

	items := make([]list.Item, len(options))
	for i, name := range options {
		items[i] = item{title: name}
	}

	l := list.New(items, itemDelegate{}, 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = lipgloss.NewStyle().
		MarginLeft(2).
		Foreground(lipgloss.Color("170")).
		Bold(true)

	m := selectorModel{list: l}
	m.list.SetSize(40, len(options)+4)

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("UIの実行に失敗しました: %w", err)
	}

	fm := finalModel.(selectorModel)
	if fm.quitting && fm.choice == "" {
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
		switch msg.String() {
		case "q", "ctrl+c":
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

		case " ", "x", "tab":
			// スペース/x/tabで選択をトグル
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
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m multiSelectModel) View() string {
	if m.quitting && m.selected == nil {
		return ""
	}

	var b strings.Builder
	titleStyle := lipgloss.NewStyle().
		MarginLeft(2).
		Foreground(lipgloss.Color("170")).
		Bold(true)
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

		style := lipgloss.NewStyle().PaddingLeft(2)
		if m.cursor == i {
			style = style.Foreground(lipgloss.Color("170"))
		}

		line := fmt.Sprintf("%s%s %s", cursor, checked, option)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	helpStyle := lipgloss.NewStyle().
		MarginTop(1).
		MarginLeft(2).
		Foreground(lipgloss.Color("241"))
	b.WriteString(helpStyle.Render("x/スペース: 選択  a: 全選択  Enter: 確定  q: キャンセル"))

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

	if len(result) == 0 {
		return nil, errors.New("項目が選択されていません")
	}

	return result, nil
}
