package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marshallshelly/pebble-orm/pkg/migration"
)

// MigrateMode represents the current mode of the migration UI
type MigrateMode int

const (
	ModeList MigrateMode = iota
	ModeConfirm
	ModeExecuting
	ModeComplete
	ModeError
)

// MigrateModel is the main Bubbletea model for interactive migrations
type MigrateModel struct {
	mode          MigrateMode
	action        string // "up" or "down"
	list          list.Model
	confirmation  ConfirmationDialog
	progress      ProgressView
	logs          LogView
	err           error
	width         int
	height        int
	dbURL         string
	migrationsDir string
	migrations    []migration.Migration
	status        []migration.MigrationRecord
	pool          *pgxpool.Pool
	executor      *migration.Executor
	selectedItems []int
}

// NewMigrateModel creates a new migration UI model
func NewMigrateModel(action, dbURL, migrationsDir string) MigrateModel {
	items := []list.Item{}
	delegate := MigrationItemDelegate{}

	l := list.New(items, delegate, 0, 0)
	l.Title = "Database Migrations"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return MigrateModel{
		mode:          ModeList,
		action:        action,
		list:          l,
		logs:          NewLogView(10),
		dbURL:         dbURL,
		migrationsDir: migrationsDir,
		selectedItems: []int{},
	}
}

// Init initializes the model
func (m MigrateModel) Init() tea.Cmd {
	return tea.Batch(
		loadMigrationsCmd(m.dbURL, m.migrationsDir),
		tea.EnterAltScreen,
	)
}

// Messages
type migrationsLoadedMsg struct {
	migrations []migration.Migration
	status     []migration.MigrationRecord
	pool       *pgxpool.Pool
	executor   *migration.Executor
}

type migrationExecutedMsg struct {
	version string
	err     error
}

type allMigrationsCompleteMsg struct{}

type errorMsg struct {
	err error
}

// Commands
func loadMigrationsCmd(dbURL, migrationsDir string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to connect to database: %w", err)}
		}

		executor := migration.NewExecutor(pool, migrationsDir)

		if err := executor.Initialize(ctx); err != nil {
			pool.Close()
			return errorMsg{err: fmt.Errorf("failed to initialize migrations: %w", err)}
		}

		generator := migration.NewGenerator(migrationsDir)
		migrationFiles, err := generator.ListMigrations()
		if err != nil {
			pool.Close()
			return errorMsg{err: fmt.Errorf("failed to list migrations: %w", err)}
		}

		var migrations []migration.Migration
		for _, file := range migrationFiles {
			mig, err := generator.ReadMigration(file)
			if err != nil {
				pool.Close()
				return errorMsg{err: fmt.Errorf("failed to read migration: %w", err)}
			}
			migrations = append(migrations, *mig)
		}

		status, err := executor.GetStatus(ctx, migrations)
		if err != nil {
			pool.Close()
			return errorMsg{err: fmt.Errorf("failed to get migration status: %w", err)}
		}

		return migrationsLoadedMsg{
			migrations: migrations,
			status:     status,
			pool:       pool,
			executor:   executor,
		}
	}
}

func executeMigrationCmd(executor *migration.Executor, mig migration.Migration, action string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		var err error
		if action == "up" {
			err = executor.Apply(ctx, mig, false)
		} else {
			err = executor.Rollback(ctx, mig, false)
		}

		return migrationExecutedMsg{
			version: mig.Version,
			err:     err,
		}
	}
}

// Update handles messages
func (m MigrateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-8)
		return m, nil

	case migrationsLoadedMsg:
		m.migrations = msg.migrations
		m.status = msg.status
		m.pool = msg.pool
		m.executor = msg.executor

		// Convert status to list items
		items := make([]list.Item, len(msg.status))
		for i, s := range msg.status {
			appliedAt := ""
			if s.AppliedAt != nil {
				appliedAt = s.AppliedAt.Format("2006-01-02 15:04:05")
			}
			items[i] = MigrationItem{
				Version:   s.Version,
				Name:      s.Name,
				Status:    string(s.Status),
				AppliedAt: appliedAt,
			}
		}
		m.list.SetItems(items)

		return m, nil

	case migrationExecutedMsg:
		if msg.err != nil {
			m.mode = ModeError
			m.err = msg.err
			m.logs.AddLog(errorStyle.Render("Failed: " + msg.version + " - " + msg.err.Error()))
			return m, nil
		}

		m.logs.AddLog(successStyle.Render("✓ Completed: " + msg.version))
		m.progress.Current++

		// Check if we're done
		if m.progress.Current >= m.progress.Total {
			m.mode = ModeComplete
			return m, nil
		}

		// Execute next migration
		nextIdx := m.selectedItems[m.progress.Current]
		nextMig := m.migrations[nextIdx]
		m.progress.Message = fmt.Sprintf("Executing: %s - %s", nextMig.Version, nextMig.Name)

		return m, executeMigrationCmd(m.executor, nextMig, m.action)

	case errorMsg:
		m.mode = ModeError
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case ModeList:
			switch msg.String() {
			case "ctrl+c", "q":
				if m.pool != nil {
					m.pool.Close()
				}
				return m, tea.Quit

			case "enter", " ":
				// Check if any migrations can be executed
				canExecute := false
				selectedIdx := m.list.Index()

				if m.action == "up" {
					// Can apply if pending
					if m.status[selectedIdx].Status == migration.StatusPending {
						canExecute = true
					}
				} else {
					// Can rollback if applied
					if m.status[selectedIdx].Status == migration.StatusApplied {
						canExecute = true
					}
				}

				if !canExecute {
					return m, nil
				}

				m.selectedItems = []int{selectedIdx}
				m.confirmation = NewConfirmationDialog(
					fmt.Sprintf("Confirm Migration %s", strings.ToUpper(m.action)),
					fmt.Sprintf("Are you sure you want to %s migration:\n%s - %s",
						m.action,
						m.status[selectedIdx].Version,
						m.status[selectedIdx].Name,
					),
				)
				m.confirmation.OnConfirm = func() tea.Cmd {
					m.mode = ModeExecuting
					m.progress = ProgressView{
						Current: 0,
						Total:   len(m.selectedItems),
						Message: fmt.Sprintf("Executing: %s - %s",
							m.migrations[m.selectedItems[0]].Version,
							m.migrations[m.selectedItems[0]].Name,
						),
					}

					// Acquire lock
					ctx := context.Background()
					if err := m.executor.Lock(ctx); err != nil {
						return func() tea.Msg {
							return errorMsg{err: fmt.Errorf("failed to acquire lock: %w", err)}
						}
					}

					return executeMigrationCmd(m.executor, m.migrations[m.selectedItems[0]], m.action)
				}
				m.confirmation.OnCancel = func() tea.Cmd {
					m.mode = ModeList
					return nil
				}
				m.mode = ModeConfirm

				return m, nil
			}

		case ModeConfirm:
			switch msg.String() {
			case "ctrl+c", "q", "esc":
				m.mode = ModeList
				return m, nil
			default:
				return m, m.confirmation.Update(msg)
			}

		case ModeComplete, ModeError:
			switch msg.String() {
			case "ctrl+c", "q", "enter":
				if m.pool != nil {
					ctx := context.Background()
					_ = m.executor.Unlock(ctx)
					m.pool.Close()
				}
				return m, tea.Quit
			}
		}
	}

	// Update list
	if m.mode == ModeList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the UI
func (m MigrateModel) View() string {
	switch m.mode {
	case ModeList:
		help := helpStyle.Render(
			FormatKey("↑/↓", "navigate") + " • " +
				FormatKey("enter", "execute") + " • " +
				FormatKey("q", "quit"),
		)
		return lipgloss.JoinVertical(lipgloss.Left,
			m.list.View(),
			help,
		)

	case ModeConfirm:
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			m.confirmation.View(),
		)

	case ModeExecuting:
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(
				lipgloss.Left,
				m.progress.View(),
				"\n",
				m.logs.View(),
			),
		)

	case ModeComplete:
		msg := titleStyle.Render("Migration Complete!") + "\n\n" +
			successStyle.Render(fmt.Sprintf("Successfully executed %d migration(s)", m.progress.Total)) + "\n\n" +
			helpStyle.Render(FormatKey("enter/q", "exit"))

		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			boxStyle.Render(msg),
		)

	case ModeError:
		msg := titleStyle.Render("Migration Failed") + "\n\n" +
			errorStyle.Render(m.err.Error()) + "\n\n" +
			helpStyle.Render(FormatKey("enter/q", "exit"))

		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			boxStyle.Render(msg),
		)
	}

	return "Unknown mode"
}

// RunMigrateUI starts the interactive migration UI
func RunMigrateUI(action, dbURL, migrationsDir string) error {
	p := tea.NewProgram(NewMigrateModel(action, dbURL, migrationsDir))
	_, err := p.Run()
	return err
}
