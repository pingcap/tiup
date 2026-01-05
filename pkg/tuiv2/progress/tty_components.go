package progress

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ttyRenderContext contains shared rendering dependencies for composing the TTY
// Active area.
type ttyRenderContext struct {
	styles ttyStyles

	width  int
	height int

	// spinner is the already-styled spinner glyph used for running tasks.
	spinner string
}

type ttyGroupComponent struct {
	group *Group
}

func (c ttyGroupComponent) Lines(ctx ttyRenderContext, activeLimit int) []string {
	g := c.group
	if g == nil {
		return nil
	}

	tasks := g.tasks
	if g.sortTasksByTitle && len(tasks) > 1 {
		tasks = append([]*Task(nil), tasks...)
		sort.SliceStable(tasks, func(i, j int) bool {
			ti := tasks[i]
			tj := tasks[j]
			if ti == nil || tj == nil {
				return ti != nil
			}
			return strings.ToLower(ti.title) < strings.ToLower(tj.title)
		})
	}

	active := 0
	hasError := false
	for _, t := range tasks {
		if t == nil {
			continue
		}
		if t.status == taskStatusRunning {
			active++
		}
		if t.status == taskStatusError {
			hasError = true
		}
	}

	meta := formatElapsed(g.elapsedLocked())

	header := g.title
	if g.showMeta {
		header += "  " + ctx.styles.meta.Render(meta)
	}
	// When meta is shown, elapsed timing already indicates activity.
	// Keep the header minimal by only appending "..." for groups that hide meta
	// (e.g. shutdown).
	if active > 0 && !g.showMeta {
		header += " ..."
	}

	// Group status icon:
	// - running: gray bullet
	// - success: green check
	// - error:   red cross
	//
	// A group is considered "finished" only when it's explicitly closed and has
	// no running tasks. This keeps the output stable even when tasks complete
	// early but the stage is still in progress.
	icon := ctx.styles.groupRunningIcon.Render("•")
	if g.closed && active == 0 {
		if hasError {
			icon = ctx.styles.groupErrorIcon.Render("✘")
		} else {
			icon = ctx.styles.groupSuccessIcon.Render("✔︎")
		}
	}

	lines := []string{ctx.styles.clipLine(ctx.width, icon+" "+header)}

	// When the group is successful and configured to hide details, render only
	// the summary line.
	if g.closed && active == 0 && !hasError && g.hideDetailsOnSuccess {
		return lines
	}

	guide := ctx.styles.guideRunning
	// When a successful group keeps details, use a green guide bar to show the
	// block as a cohesive "completed section" (similar to docker build output).
	if g.closed && active == 0 && !hasError && !g.hideDetailsOnSuccess {
		guide = ctx.styles.guideSuccess
	}

	shown := len(tasks)
	if activeLimit >= 0 && shown > activeLimit {
		shown = activeLimit
	}

	// Compute per-group column widths so task metadata/messages line up vertically.
	//
	// We intentionally only consider the tasks we're going to show, so hidden
	// tasks (e.g. "... and N more") won't affect alignment.
	maxTitleWidth := 0
	for i := 0; i < shown; i++ {
		t := tasks[i]
		if t == nil {
			continue
		}
		// Align tasks that show a second column (meta/message), error details,
		// or download metadata/progress.
		if t.kind == taskKindDownload || t.meta != "" || t.message != "" || t.status == taskStatusError {
			if w := lipgloss.Width(t.title); w > maxTitleWidth {
				maxTitleWidth = w
			}
		}
	}

	maxDownloadLabelWidth := 0
	if maxTitleWidth > 0 {
		for i := 0; i < shown; i++ {
			t := tasks[i]
			if t == nil || t.kind != taskKindDownload {
				continue
			}
			label := ttyDownloadLabel(t, ctx, maxTitleWidth)
			if w := lipgloss.Width(label); w > maxDownloadLabelWidth {
				maxDownloadLabelWidth = w
			}
		}
	}

	for i := 0; i < shown; i++ {
		lines = append(lines, ttyTaskComponent{
			task:               tasks[i],
			guide:              guide,
			titleWidth:         maxTitleWidth,
			downloadLabelWidth: maxDownloadLabelWidth,
		}.Line(ctx))
	}
	if len(tasks) > shown {
		lines = append(lines, ctx.styles.clipLine(ctx.width, fmt.Sprintf("  … and %d more", len(tasks)-shown)))
	}

	return lines
}

type ttyTaskComponent struct {
	task  *Task
	guide lipgloss.Style

	titleWidth         int
	downloadLabelWidth int
}

func (c ttyTaskComponent) Line(ctx ttyRenderContext) string {
	t := c.task
	if t == nil {
		return ""
	}

	var symbol string
	switch t.status {
	case taskStatusPending:
		symbol = ctx.styles.taskPendingIcon.Render("·")
	case taskStatusRunning:
		symbol = ctx.spinner
	case taskStatusDone:
		symbol = ctx.styles.taskSuccessIcon.Render("✔︎")
	case taskStatusError:
		symbol = ctx.styles.taskErrorIcon.Render("✘")
	case taskStatusSkipped:
		symbol = ctx.styles.taskSkippedIcon.Render("↷")
	case taskStatusCanceled:
		symbol = ctx.styles.taskCanceledIcon.Render("!")
	default:
		symbol = "-"
	}

	guideBar := c.guide.Render("┃")

	// Prefix format (stable, line-oriented):
	//   "  ┃  ✔︎ "
	prefix := "  " + guideBar + "  " + symbol + " "
	prefixWidth := lipgloss.Width(prefix)

	content := ""
	switch {
	case t.kind == taskKindDownload:
		content = ttyDownloadContent(t, ctx, c.titleWidth, c.downloadLabelWidth)
	case t.status == taskStatusError:
		if t.meta == "" && t.message != "" {
			// When a task errors without stable metadata, align the error message to
			// the second column for consistency and to maximize space on narrow
			// terminals.
			title := ttyTaskLabel(t, ctx, c.titleWidth)
			content = title + " " + t.message
		} else {
			titleWidth := 0
			if t.meta != "" {
				titleWidth = c.titleWidth
			}
			title := ttyTaskLabel(t, ctx, titleWidth)
			if t.message != "" {
				// Keep the colon adjacent to the title for readability. Padding the
				// title would introduce awkward spaces before ':' when other tasks have
				// longer titles (e.g. "TiKV Worker").
				content = fmt.Sprintf("%s: %s", title, t.message)
			} else {
				content = title
			}
		}
	case t.status == taskStatusSkipped:
		titleWidth := 0
		if t.meta != "" {
			titleWidth = c.titleWidth
		}
		title := ttyTaskLabel(t, ctx, titleWidth)
		if t.message != "" {
			content = fmt.Sprintf("%s: %s", title, ctx.styles.message.Render(t.message))
		} else {
			content = title
		}
	case t.status == taskStatusCanceled:
		titleWidth := 0
		if t.meta != "" {
			titleWidth = c.titleWidth
		}
		title := ttyTaskLabel(t, ctx, titleWidth)
		if t.message != "" {
			content = fmt.Sprintf("%s: %s", title, ctx.styles.message.Render(t.message))
		} else {
			content = title
		}
	case t.message != "":
		title := ttyTaskLabel(t, ctx, c.titleWidth)
		content = title + "  " + ctx.styles.message.Render(t.message)
	default:
		if t.meta != "" {
			content = ttyTaskLabel(t, ctx, c.titleWidth)
		} else {
			content = ttyTaskLabel(t, ctx, 0)
		}
	}

	// Ensure the prefix is always visible, and clip the content into remaining
	// width. This avoids truncating ANSI sequences.
	if ctx.width > 0 && prefixWidth >= ctx.width {
		return ctx.styles.clipLine(ctx.width, prefix)
	}

	if ctx.width > 0 {
		maxContent := ctx.width - prefixWidth
		content = ctx.styles.clipLine(maxContent, content)
	}

	return ctx.styles.clipLine(ctx.width, prefix+content)
}

func padRightVisible(s string, width int) string {
	if width <= 0 || s == "" {
		return s
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func ttyTaskLabel(t *Task, ctx ttyRenderContext, titleWidth int) string {
	if t == nil {
		return ""
	}

	title := t.title
	if titleWidth > 0 {
		title = padRightVisible(title, titleWidth)
	}
	if t.meta != "" {
		title += " " + ctx.styles.meta.Render(t.meta)
	}
	return title
}

func ttyDownloadLabel(t *Task, ctx ttyRenderContext, titleWidth int) string {
	if t == nil {
		return ""
	}
	label := ttyTaskLabel(t, ctx, titleWidth)
	if meta := ttyDownloadMeta(t); meta != "" {
		label += " " + ctx.styles.meta.Render(meta)
	}
	return label
}

func ttyDownloadContent(t *Task, ctx ttyRenderContext, titleWidth, labelWidth int) string {
	label := ttyDownloadLabel(t, ctx, titleWidth)

	switch t.status {
	case taskStatusRunning:
		if labelWidth > 0 {
			label = padRightVisible(label, labelWidth)
		}
		parts := []string{label}
		if t.total > 0 {
			percent := int64(0)
			if t.current > 0 {
				percent = t.current * 100 / t.total
			}

			// Keep the progress bar compact: it's nice on wide terminals but should
			// disappear first when space is limited.
			bar := ""
			switch {
			case ctx.width >= 70:
				bar = renderProgressBar(ctx.styles, t.current, t.total, 18)
			case ctx.width >= 55:
				bar = renderProgressBar(ctx.styles, t.current, t.total, 12)
			}
			if bar != "" {
				parts = append(parts, bar)
			}

			parts = append(parts, fmt.Sprintf("%d%%", percent))
			if t.speedBps > 0 {
				parts = append(parts, ctx.styles.meta.Render(fmt.Sprintf("(%s)", formatSpeed(t.speedBps))))
			}
		} else if t.current > 0 {
			parts = append(parts, formatBytes(t.current))
			if t.speedBps > 0 {
				parts = append(parts, ctx.styles.meta.Render(fmt.Sprintf("(%s)", formatSpeed(t.speedBps))))
			}
		}
		return strings.Join(parts, "  ")
	case taskStatusDone:
		return label
	case taskStatusError:
		if t.message != "" {
			return fmt.Sprintf("%s: %s", label, t.message)
		}
		return fmt.Sprintf("%s: failed", label)
	default:
		return label
	}
}

func ttyDownloadMeta(t *Task) string {
	if t == nil {
		return ""
	}

	parts := make([]string, 0, 1)
	if t.total > 0 {
		parts = append(parts, fmt.Sprintf("(%s)", formatBytes(t.total)))
	} else if t.status != taskStatusRunning && t.current > 0 {
		// Best-effort: if total is unknown but we already finished, use the final
		// byte count as a stable size hint.
		parts = append(parts, fmt.Sprintf("(%s)", formatBytes(t.current)))
	}
	return strings.Join(parts, " ")
}

func renderProgressBar(styles ttyStyles, current, total int64, width int) string {
	if width <= 0 || total <= 0 {
		return ""
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	filled := int(float64(current) / float64(total) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	bar := styles.progressFilled.Render(strings.Repeat("━", filled)) + styles.progressTrack.Render(strings.Repeat("━", width-filled))
	return bar
}

func renderTTYBlocksLocked(groups []*Group, ctx ttyRenderContext, activeLimit int) [][]string {
	var blocks [][]string
	for _, g := range groups {
		if g == nil || g.sealed {
			continue
		}
		if len(g.tasks) == 0 {
			continue
		}
		blocks = append(blocks, ttyGroupComponent{group: g}.Lines(ctx, activeLimit))
	}
	return blocks
}

func flattenBlocks(blocks [][]string) []string {
	var lines []string
	for _, b := range blocks {
		lines = append(lines, b...)
	}
	return lines
}
