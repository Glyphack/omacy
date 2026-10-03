package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

const (
	// appGroups is how many groups at the start of Config.groups are apps. The rest are macOS settings.
	appGroups = 2
	zoneGap   = 3
	// minAppsWidth is the narrowest the apps zone gets before the macOS zone moves under it.
	minAppsWidth = 40
	maxAppsWidth = 80
	// headerLines is the tab row, the line under the active tab and a blank line.
	headerLines   = 3
	helpLines     = 2
	optionIndent  = len("> [x] ")
	settingIndent = len("    > [x] ")
	// countWidth fits a count like 16/17.
	countWidth = 5
	noGroup    = -1
)

type formStyles struct {
	tab, activeTab, zone, group, cursor, tick, partial, changed, note, help, hint, question, rule lipgloss.Style
}

func newFormStyles(dark bool) formStyles {
	ld := lipgloss.LightDark(dark)
	accent := ld(lipgloss.Color("#5A56E0"), lipgloss.Color("#9F9CFF"))
	green := ld(lipgloss.Color("#1E8A3A"), lipgloss.Color("#5FD787"))
	yellow := ld(lipgloss.Color("#A15C00"), lipgloss.Color("#FFD75F"))
	faint := ld(lipgloss.Color("#767676"), lipgloss.Color("#8A8A8A"))
	return formStyles{
		tab:       lipgloss.NewStyle().Foreground(faint),
		activeTab: lipgloss.NewStyle().Foreground(accent).Bold(true),
		zone:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		group:     lipgloss.NewStyle().Bold(true),
		cursor:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		tick:      lipgloss.NewStyle().Foreground(green),
		partial:   lipgloss.NewStyle().Foreground(yellow),
		changed:   lipgloss.NewStyle().Foreground(yellow).Bold(true),
		note:      lipgloss.NewStyle().Foreground(faint),
		help:      lipgloss.NewStyle().Italic(true),
		hint:      lipgloss.NewStyle().Foreground(faint),
		question:  lipgloss.NewStyle().Foreground(accent).Bold(true),
		rule:      lipgloss.NewStyle().Foreground(faint),
	}
}

func runConfigForm(args arguments) Config {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "omacy asks its questions in a terminal. Without one, run it again with --no-interactive.")
		os.Exit(2)
	}

	dark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	final, err := tea.NewProgram(newConfigForm(args, newFormStyles(dark))).Run()
	if err != nil {
		fail("Cannot show the setup form", err)
	}
	config, ok := final.(*configForm).chosen()
	if !ok {
		stopped()
	}
	return config
}

type question int

const (
	noQuestion question = iota
	askSwitch
	askStart
)

type zone int

const (
	appsZone zone = iota
	macZone
)

// place is one row of the form. option is -1 on the line of a macOS group.
type place struct{ group, option int }

func (p place) zone() zone {
	if p.group < appGroups {
		return appsZone
	}
	return macZone
}

func (p place) isGroupLine() bool { return p.option < 0 }

// configForm is the start form. A row of tabs, one per profile, sits on top. The apps zone shows
// every app option with its help. The macOS zone shows one line per group and the settings of the
// one open group.
type configForm struct {
	args          arguments
	styles        formStyles
	tab           int // index in configProfiles
	config        Config
	groups        []optionGroup
	base          []optionGroup // the options before any edit, to mark what changed
	cursor        place
	open          int    // the open macOS group, or noGroup
	tops          [2]int // first line on screen of each pane
	ask           question
	switchTo      int
	width, height int
	done          bool
	start         bool
}

func newConfigForm(args arguments, styles formStyles) *configForm {
	m := &configForm{args: args, styles: styles, open: noGroup}
	m.fill(args.profile)
	return m
}

func (m *configForm) fill(tab int) {
	m.tab = tab
	m.config = m.args.config(configProfiles[tab])
	m.groups = m.config.groups()
	fresh := m.args.config(configProfiles[tab])
	m.base = fresh.groups()
}

func (m *configForm) chosen() (Config, bool) {
	return m.config, m.start
}

func (m *configForm) option(p place) *option {
	return m.groups[p.group].options[p.option]
}

func (m *configForm) changed(p place) bool {
	if !p.isGroupLine() {
		return m.option(p).on != m.base[p.group].options[p.option].on
	}
	for i := range m.groups[p.group].options {
		if m.changed(place{p.group, i}) {
			return true
		}
	}
	return false
}

func (m *configForm) edited() bool {
	for g := range m.groups {
		if m.changed(place{g, -1}) {
			return true
		}
	}
	return false
}

func (m *configForm) count(g int) (on, total int) {
	for _, o := range m.groups[g].options {
		if o.on {
			on++
		}
	}
	return on, len(m.groups[g].options)
}

func (m *configForm) tickGroup(g int) {
	on, total := m.count(g)
	for _, o := range m.groups[g].options {
		o.on = on < total
	}
}

func (m *configForm) rows() []place {
	var rows []place
	for g, group := range m.groups {
		if g >= appGroups {
			rows = append(rows, place{g, -1})
			if g != m.open {
				continue
			}
		}
		for i := range group.options {
			rows = append(rows, place{g, i})
		}
	}
	return rows
}

func (m *configForm) Init() tea.Cmd { return nil }

func (m *configForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		m.press(msg.String())
	}
	if m.done {
		return m, tea.Quit
	}
	m.follow()
	return m, nil
}

func (m *configForm) press(k string) {
	switch k {
	case "esc", "q", "ctrl+c":
		m.done = true
		return
	}
	if m.ask != noQuestion {
		m.answer(k)
		return
	}
	rows := m.rows()
	at := slices.Index(rows, m.cursor)
	switch k {
	case "tab":
		m.pickProfile((m.tab + 1) % len(configProfiles))
	case "shift+tab":
		m.pickProfile((m.tab + len(configProfiles) - 1) % len(configProfiles))
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if tab := int(k[0] - '1'); tab < len(configProfiles) {
			m.pickProfile(tab)
		}
	case "j", "down":
		m.cursor = rows[min(at+1, len(rows)-1)]
	case "k", "up":
		m.cursor = rows[max(at-1, 0)]
	case "h", "left":
		m.jump(appsZone)
	case "l", "right":
		m.jump(macZone)
	case "space", "x":
		if m.cursor.isGroupLine() {
			m.tickGroup(m.cursor.group)
			return
		}
		m.option(m.cursor).on = !m.option(m.cursor).on
	case "a":
		m.tickGroup(m.cursor.group)
	case "enter":
		if !m.cursor.isGroupLine() {
			m.ask = askStart
			return
		}
		if m.open == m.cursor.group {
			m.open = noGroup
			return
		}
		m.open = m.cursor.group
	}
}

func (m *configForm) answer(k string) {
	switch k {
	case "y", "Y":
		if m.ask == askStart {
			m.start = true
			m.done = true
			return
		}
		m.fill(m.switchTo)
		m.ask = noQuestion
	case "n", "N":
		m.ask = noQuestion
	}
}

func (m *configForm) pickProfile(tab int) {
	if tab == m.tab {
		return
	}
	if m.edited() {
		m.ask = askSwitch
		m.switchTo = tab
		return
	}
	m.fill(tab)
}

// jump moves the cursor into zone z, to the row whose lines on screen are nearest the cursor.
func (m *configForm) jump(z zone) {
	if m.cursor.zone() == z {
		return
	}
	l := m.layout()
	from := 0
	for i, p := range l.panes {
		if s, ok := p.find(m.cursor); ok {
			from = s.line - m.tops[i]
		}
	}
	best, bestDist := m.cursor, -1
	for i, p := range l.panes {
		for _, s := range p.spans {
			dist := max(s.first-m.tops[i]-from, from-s.last+m.tops[i], 0)
			if s.at.zone() == z && (bestDist < 0 || dist < bestDist) {
				best, bestDist = s.at, dist
			}
		}
	}
	m.cursor = best
}

// span is where a row sits in a pane. line is the row itself, and first to last are the lines
// that belong with it: the headings above it and its help below it.
type span struct {
	at                place
	first, line, last int
}

// pane is a list of lines that scrolls on its own.
type pane struct {
	lines []string
	spans []span
}

// add appends a line that belongs to the next row.
func (p *pane) add(line string) { p.lines = append(p.lines, line) }

func (p *pane) row(at place, line string) {
	first := 0
	if n := len(p.spans); n > 0 {
		first = p.spans[n-1].last + 1
	}
	p.spans = append(p.spans, span{at, first, len(p.lines), len(p.lines)})
	p.add(line)
}

// more appends a line that belongs to the last row.
func (p *pane) more(line string) {
	p.spans[len(p.spans)-1].last = len(p.lines)
	p.add(line)
}

func (p *pane) join(q pane) {
	shift := len(p.lines)
	for _, s := range q.spans {
		p.spans = append(p.spans, span{s.at, s.first + shift, s.line + shift, s.last + shift})
	}
	p.lines = append(p.lines, q.lines...)
}

func (p pane) find(at place) (span, bool) {
	for _, s := range p.spans {
		if s.at == at {
			return s, true
		}
	}
	return span{}, false
}

// layout is the body: two panes side by side, or one pane with the macOS zone under the apps zone.
type layout struct {
	panes  []pane
	widths []int
}

func (m *configForm) layout() layout {
	macWidth := m.macWidth()
	if m.width >= minAppsWidth+zoneGap+macWidth {
		appsWidth := min(m.width-zoneGap-macWidth, maxAppsWidth)
		return layout{[]pane{m.appsPane(appsWidth), m.macPane(macWidth)}, []int{appsWidth, macWidth}}
	}
	width := min(m.width, maxAppsWidth)
	p := m.appsPane(width)
	p.add("")
	p.join(m.macPane(width))
	return layout{[]pane{p}, []int{width}}
}

// macWidth is the width of the macOS zone, wide enough for every group with its settings open.
func (m *configForm) macWidth() int {
	width := lipgloss.Width("> [x] ▸ ") + m.groupNameWidth() + 2 + countWidth + len(" *")
	for _, group := range m.groups[appGroups:] {
		for _, o := range group.options {
			width = max(width, settingIndent+lipgloss.Width(o.title)+len(" *"))
		}
	}
	return width
}

func (m *configForm) groupNameWidth() int {
	width := 0
	for _, group := range m.groups[appGroups:] {
		width = max(width, lipgloss.Width(group.name))
	}
	return width
}

func (m *configForm) zoneTitle(name string, width int) string {
	return m.styles.zone.Render(name) + " " + m.styles.rule.Render(strings.Repeat("─", max(width-lipgloss.Width(name)-1, 0)))
}

func (m *configForm) appsPane(width int) pane {
	var p pane
	p.add(m.zoneTitle("Apps", width))
	for g := range appGroups {
		p.add("")
		p.add(m.styles.group.Render(m.groups[g].name))
		for i, o := range m.groups[g].options {
			p.row(place{g, i}, m.optionLine(place{g, i}, ""))
			for _, line := range strings.Split(lipgloss.Wrap(o.help, width-optionIndent, ""), "\n") {
				p.more(strings.Repeat(" ", optionIndent) + m.styles.note.Render(line))
			}
		}
	}
	return p
}

func (m *configForm) macPane(width int) pane {
	var p pane
	p.add(m.zoneTitle("macOS settings", width))
	p.add("")
	for g := appGroups; g < len(m.groups); g++ {
		p.row(place{g, -1}, m.groupLine(g))
		if g != m.open {
			continue
		}
		for i := range m.groups[g].options {
			p.row(place{g, i}, m.optionLine(place{g, i}, "    "))
		}
	}
	return p
}

func (m *configForm) box(p place) string {
	if !p.isGroupLine() {
		if m.option(p).on {
			return m.styles.tick.Render("[x]")
		}
		return "[ ]"
	}
	on, total := m.count(p.group)
	switch on {
	case total:
		return m.styles.tick.Render("[x]")
	case 0:
		return "[ ]"
	}
	return m.styles.partial.Render("[-]")
}

func (m *configForm) mark(p place, title string) (string, string) {
	if p != m.cursor {
		return "  ", title
	}
	return m.styles.cursor.Render(">") + " ", m.styles.cursor.Render(title)
}

func (m *configForm) star(p place) string {
	if !m.changed(p) {
		return ""
	}
	return " " + m.styles.changed.Render("*")
}

func (m *configForm) optionLine(p place, indent string) string {
	mark, title := m.mark(p, m.option(p).title)
	return indent + mark + m.box(p) + " " + title + m.star(p)
}

func (m *configForm) groupLine(g int) string {
	p := place{g, -1}
	name := m.groups[g].name
	name += strings.Repeat(" ", m.groupNameWidth()-lipgloss.Width(name))
	mark, name := m.mark(p, name)
	arrow := "▸"
	if g == m.open {
		arrow = "▾"
	}
	on, total := m.count(g)
	count := fmt.Sprintf("%*s", countWidth, fmt.Sprintf("%d/%d", on, total))
	return mark + m.box(p) + " " + arrow + " " + name + "  " + m.styles.note.Render(count) + m.star(p)
}

// follow scrolls each pane so the row under the cursor is on screen with its headings and help.
func (m *configForm) follow() {
	l := m.layout()
	h := m.bodyHeight()
	for i, p := range l.panes {
		if s, ok := p.find(m.cursor); ok {
			last := min(s.last, s.first+h-1)
			if s.first < m.tops[i] {
				m.tops[i] = s.first
			}
			if last >= m.tops[i]+h {
				m.tops[i] = last - h + 1
			}
		}
		m.tops[i] = max(min(m.tops[i], len(p.lines)-h), 0)
	}
}

func (m *configForm) hints() []string {
	items := []string{"↑↓←→ hjkl move", "space tick", "a group", "enter open group/start", "tab profile", "q quit"}
	switch m.ask {
	case askStart:
		items = []string{"y start", "n back", "q quit"}
	case askSwitch:
		items = []string{"y switch profile", "n back", "q quit"}
	}
	var lines []string
	line := ""
	for _, item := range items {
		if line != "" && lipgloss.Width(line+"  "+item) > m.width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += "  "
		}
		line += item
	}
	return append(lines, line)
}

func (m *configForm) bodyHeight() int {
	return max(m.height-headerLines-1-helpLines-len(m.hints()), 1)
}

func (m *configForm) tabRow() (string, string) {
	var row, under string
	for i, profile := range configProfiles {
		label := string(rune('1'+i)) + " " + profile.title
		style, mark := m.styles.tab, " "
		if i == m.tab {
			style, mark = m.styles.activeTab, "─"
			if m.edited() {
				label += " *"
			}
		}
		row += " " + style.Render(label) + "  "
		under += " " + style.Render(strings.Repeat(mark, lipgloss.Width(label))) + "  "
	}
	return strings.TrimRight(row, " "), strings.TrimRight(under, " ")
}

func (m *configForm) body(l layout) []string {
	var lines []string
	for r := range m.bodyHeight() {
		row := ""
		for i, p := range l.panes {
			line := ""
			if n := m.tops[i] + r; n < len(p.lines) {
				line = p.lines[n]
			}
			row += line + strings.Repeat(" ", max(l.widths[i]-lipgloss.Width(line), 0)+zoneGap)
		}
		lines = append(lines, strings.TrimRight(row, " "))
	}
	return lines
}

func (m *configForm) rule(l layout) string {
	above, below := false, false
	for i, p := range l.panes {
		above = above || m.tops[i] > 0
		below = below || m.tops[i]+m.bodyHeight() < len(p.lines)
	}
	text := ""
	if above {
		text += "── more above "
	}
	if below {
		text += "── more below "
	}
	return m.styles.rule.Render(text + strings.Repeat("─", max(m.width-lipgloss.Width(text), 0)))
}

// help gives the flag for app options, since their help is already shown in the apps zone.
func (m *configForm) help() string {
	switch m.ask {
	case askStart:
		return m.styles.question.Render("Start the setup? (y/n)")
	case askSwitch:
		return m.styles.question.Render("Throw away your changes? (y/n)")
	}
	p := m.cursor
	switch {
	case p.isGroupLine():
		on, total := m.count(p.group)
		action := "enter shows them"
		if p.group == m.open {
			action = "enter hides them"
		}
		return m.styles.help.Render(fmt.Sprintf("%d of %d %s settings are on, %s.", on, total, m.groups[p.group].name, action))
	case p.zone() == appsZone:
		flag := "--" + m.option(p).flag
		return m.styles.note.Render("On the command line: " + flag + " or " + flag + "=false")
	}
	return m.styles.help.Render(m.option(p).help)
}

func (m *configForm) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	if m.done || m.width == 0 {
		return v
	}
	clip := lipgloss.NewStyle().MaxWidth(m.width)
	l := m.layout()
	tabs, under := m.tabRow()
	lines := []string{clip.Render(tabs), clip.Render(under), ""}
	for _, line := range m.body(l) {
		lines = append(lines, clip.Render(line))
	}
	lines = append(lines,
		m.rule(l),
		lipgloss.NewStyle().Width(m.width).Height(helpLines).MaxHeight(helpLines).Render(m.help()),
	)
	for _, line := range m.hints() {
		lines = append(lines, clip.Render(m.styles.hint.Render(line)))
	}
	v.SetContent(strings.Join(lines[:min(len(lines), m.height)], "\n"))
	return v
}
