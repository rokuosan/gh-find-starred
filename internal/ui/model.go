package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rokuosan/gh-find-starred/internal/domain"
)

// Service defines the application operations required by the TUI.
type Service interface {
	Prepare(ctx context.Context, noCache bool, onProgress func(domain.FetchProgress)) (domain.IndexStatus, domain.IndexSearcher, error)
}

// Options configures the TUI session.
type Options struct {
	Query   string
	Limit   int
	NoCache bool
}

// State represents the current lifecycle stage of the TUI.
type State int

const (
	StateFetching State = iota
	StateSearching
	StateDone
	StateError
	StateCanceled
)

type fetchProgressMsg struct {
	fetchedCount int
}

type prepareDoneMsg struct {
	status   domain.IndexStatus
	searcher domain.IndexSearcher
	err      error
}

type searchDoneMsg struct {
	results []domain.SearchResult
	err     error
}

// Model is the Bubble Tea v2 model coordinating TUI state and rendering.
type Model struct {
	svc          Service
	opts         Options
	styles       Styles
	ctx          context.Context
	cancel       context.CancelFunc
	events       chan tea.Msg
	state        State
	spinner      spinner.Model
	fetchedCount int
	indexStatus  domain.IndexStatus
	searcher     domain.IndexSearcher
	results      []domain.SearchResult
	err          error
}

// NewModel constructs a TUI Model for the given Service and Options.
func NewModel(ctx context.Context, svc Service, opts Options) Model {
	if ctx == nil {
		ctx = context.Background()
	}
	childCtx, cancel := context.WithCancel(ctx)
	styles := DefaultStyles()

	return Model{
		svc:     svc,
		opts:    opts,
		styles:  styles,
		ctx:     childCtx,
		cancel:  cancel,
		events:  make(chan tea.Msg, 32),
		state:   StateFetching,
		spinner: newSpinner(spinner.Dot, styles.Spinner),
	}
}

func newSpinner(kind spinner.Spinner, style lipgloss.Style) spinner.Model {
	s := spinner.New(spinner.WithSpinner(kind))
	s.Style = style
	return s
}

// Init starts the spinner and asynchronous repository index preparation.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startPrepareCmd(),
		waitForEvent(m.ctx, m.events),
	)
}

func (m Model) startPrepareCmd() tea.Cmd {
	svc := m.svc
	ctx := m.ctx
	noCache := m.opts.NoCache
	events := m.events

	return func() tea.Msg {
		if svc == nil {
			sendFinal(ctx, events, prepareDoneMsg{err: errors.New("application service is not configured")})
			return nil
		}

		status, searcher, err := svc.Prepare(ctx, noCache, func(p domain.FetchProgress) {
			select {
			case events <- fetchProgressMsg{fetchedCount: p.FetchedCount}:
			default:
			}
		})
		sendFinal(ctx, events, prepareDoneMsg{status: status, searcher: searcher, err: err})
		return nil
	}
}

func sendFinal(ctx context.Context, ch chan<- tea.Msg, msg tea.Msg) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
}

func waitForEvent(ctx context.Context, ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-ch:
			return msg
		case <-ctx.Done():
			return nil
		}
	}
}

func (m Model) searchCmd(searcher domain.IndexSearcher) tea.Cmd {
	ctx := m.ctx
	query := m.opts.Query
	limit := m.opts.Limit

	return func() tea.Msg {
		if searcher == nil {
			return searchDoneMsg{err: errors.New("search index is not configured")}
		}
		results, err := searcher.Search(ctx, query, limit)
		return searchDoneMsg{results: results, err: err}
	}
}

// Update handles Bubble Tea messages and transitions state cleanly.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.state = StateCanceled
			m.cancel()
			return m, tea.Quit
		}

	case spinner.TickMsg:
		if m.state == StateFetching || m.state == StateSearching {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case fetchProgressMsg:
		if m.state != StateFetching {
			return m, nil
		}
		if msg.fetchedCount > m.fetchedCount {
			m.fetchedCount = msg.fetchedCount
		}
		return m, waitForEvent(m.ctx, m.events)

	case prepareDoneMsg:
		if m.state == StateCanceled {
			return m, nil
		}
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) && m.ctx.Err() != nil {
				m.state = StateCanceled
				return m, tea.Quit
			}
			m.state = StateError
			m.err = msg.err
			m.cancel()
			return m, tea.Quit
		}

		m.indexStatus = msg.status
		m.searcher = msg.searcher
		m.fetchedCount = msg.status.RepoCount
		m.state = StateSearching
		m.spinner = newSpinner(spinner.Points, m.styles.Spinner)

		return m, tea.Batch(
			m.spinner.Tick,
			m.searchCmd(msg.searcher),
		)

	case searchDoneMsg:
		if m.state == StateCanceled {
			return m, nil
		}
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) && m.ctx.Err() != nil {
				m.state = StateCanceled
				return m, tea.Quit
			}
			m.state = StateError
			m.err = msg.err
			m.cancel()
			return m, tea.Quit
		}

		m.results = msg.results
		m.state = StateDone
		m.cancel()
		return m, tea.Quit
	}

	return m, nil
}

// View renders the TUI state as a Bubble Tea v2 View.
func (m Model) View() tea.View {
	var sb strings.Builder

	switch m.state {
	case StateFetching:
		sb.WriteString(fmt.Sprintf("%s Fetching starred repositories: %d\n", m.spinner.View(), m.fetchedCount))

	case StateSearching:
		sb.WriteString(m.renderIndexSummary())
		sb.WriteString(fmt.Sprintf("%s Searching...\n", m.spinner.View()))

	case StateDone:
		sb.WriteString(m.renderIndexSummary())
		if len(m.results) == 0 {
			sb.WriteString(m.styles.Muted.Render(fmt.Sprintf("No repositories matched %q.\n", m.opts.Query)))
		} else {
			sb.WriteString("\n")
			for _, r := range m.results {
				score := m.styles.Score.Render(fmt.Sprintf("%6.2f", r.Score))
				name := m.styles.RepoName.Render(r.Repository.Name)
				url := m.styles.RepoURL.Render(r.Repository.URL)
				sb.WriteString(fmt.Sprintf("%s  %s  %s\n", score, name, url))
				if desc := singleLine(r.Repository.Description); desc != "" {
					sb.WriteString(fmt.Sprintf("        %s\n", m.styles.Description.Render(desc)))
				}
			}
		}

	case StateError:
		sb.WriteString(fmt.Sprintf("%s %v\n", m.styles.Error.Render("✗ Error:"), m.err))

	case StateCanceled:
		sb.WriteString(m.styles.Muted.Render("Canceled.\n"))
	}

	return tea.NewView(sb.String())
}

func (m Model) renderIndexSummary() string {
	var sb strings.Builder
	var summary string
	if m.indexStatus.FromCache {
		summary = fmt.Sprintf("%s Loaded starred repositories: %d %s",
			m.styles.Success.Render("✓"),
			m.indexStatus.RepoCount,
			m.styles.Muted.Render("(from cache)"),
		)
	} else {
		summary = fmt.Sprintf("%s Fetched & indexed starred repositories: %d",
			m.styles.Success.Render("✓"),
			m.indexStatus.RepoCount,
		)
	}
	sb.WriteString(summary + "\n")
	if m.indexStatus.CacheWarning != nil {
		sb.WriteString(m.styles.Warning.Render(fmt.Sprintf("⚠ Warning: failed to persist cache: %v", m.indexStatus.CacheWarning)) + "\n")
	}
	return sb.String()
}

func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// State returns the model's current State.
func (m Model) State() State {
	return m.state
}

// Err returns the terminal error if the model ended in StateError.
func (m Model) Err() error {
	return m.err
}

// Canceled reports whether the user canceled execution.
func (m Model) Canceled() bool {
	return m.state == StateCanceled
}

// Results returns the final search results.
func (m Model) Results() []domain.SearchResult {
	return m.results
}
