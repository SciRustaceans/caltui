// Package keys defines caltui's global key bindings (arrow keys + vim hjkl, plus
// mnemonic action keys) and supplies the help.KeyMap interface for the help bar.
package keys

import "charm.land/bubbles/v2/key"

// KeyMap holds every binding used across the app.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Tab     key.Binding
	PrevTab key.Binding
	Tabs    key.Binding // digit jump 1..5

	Add    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Search key.Binding
	Enter  key.Binding
	Back   key.Binding

	Help key.Binding
	Quit key.Binding
}

// Default returns the standard key bindings.
func Default() KeyMap {
	return KeyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev tab")),
		Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next tab")),
		Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
		PrevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧tab", "prev tab")),
		Tabs:    key.NewBinding(key.WithKeys("1", "2", "3", "4", "5"), key.WithHelp("1-5", "jump tab")),

		Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Search: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),

		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp is the compact help line shown at the bottom of the screen.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tabs, k.Add, k.Search, k.Help, k.Quit}
}

// FullHelp is the expanded help shown when help is toggled.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Tab, k.PrevTab, k.Tabs},
		{k.Add, k.Edit, k.Delete, k.Search, k.Enter, k.Back},
		{k.Help, k.Quit},
	}
}
