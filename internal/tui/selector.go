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
