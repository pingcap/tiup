package progress

import (
	"fmt"
	"sort"
	"strings"
	"time"

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

	now time.Time
}

type ttyGroupComponent struct {
	group *groupState
}

func ttyTaskVisible(t *taskState, now time.Time) bool {
	if t == nil {
		return false
	}
	if !t.hideIfFast {
		return true
	}
	switch t.status {
	case taskStatusError:
		return true
	case taskStatusRetrying:
		return true
	case taskStatusRunning:
		if t.revealAfter <= 0 {
			return true
		}
		if t.startAt.IsZero() {
			return true
		}
		if now.IsZero() {
			now = time.Now()
		}
		return now.Sub(t.startAt) >= t.revealAfter
	default:
		return false
	}
}

func (c ttyGroupComponent) Lines(ctx ttyRenderContext, activeLimit int) []string {
	g := c.group
	if g == nil {
		return nil
	}

	tasks := g.tasks
	if g.sortTasksByTitle && len(tasks) > 1 {
		tasks = append([]*taskState(nil), tasks...)
		sort.SliceStable(tasks, func(i, j int) bool {
			ti := tasks[i]
			tj := tasks[j]
			if ti == nil || tj == nil {
				return ti != nil
			}
			return strings.ToLower(ti.title) < strings.ToLower(tj.title)
		})
	}

	now := ctx.now
	if now.IsZero() {
		now = time.Now()
	}

	visibleTasks := make([]*taskState, 0, len(tasks))
	for _, t := range tasks {
		if ttyTaskVisible(t, now) {
			visibleTasks = append(visibleTasks, t)
		}
	}

	active := 0
	hasError := false
	for _, t := range visibleTasks {
		if t == nil {
			continue
		}
		if t.status == taskStatusRunning || t.status == taskStatusRetrying {
			active++
		}
		if t.status == taskStatusError {
			hasError = true
		}
	}

	meta := formatElapsed(g.elapsed(now))

	header := g.title
	if g.showMeta {
		header += "  " + ctx.styles.meta.Render(meta)
	}
	if active > 0 && !g.showMeta {
		header += " ..."
	}

	icon := ctx.styles.groupRunningIcon.Render("•")
	if g.closed && active == 0 {
		if hasError {
			icon = ctx.styles.groupErrorIcon.Render("✘")
		} else {
			icon = ctx.styles.groupSuccessIcon.Render("✔︎")
		}
	}

	lines := []string{ctx.styles.clipLine(ctx.width, icon+" "+header)}

	if g.closed && active == 0 && !hasError && g.hideDetailsOnSuccess {
		return lines
	}

	guide := ctx.styles.guideRunning
	if g.closed && active == 0 && !hasError && !g.hideDetailsOnSuccess {
		guide = ctx.styles.guideSuccess
	}

	shown := len(visibleTasks)
	if activeLimit >= 0 && shown > activeLimit {
		shown = activeLimit
	}

	maxTitleWidth := 0
	for i := 0; i < shown; i++ {
		t := visibleTasks[i]
		if t == nil {
			continue
		}
		if t.kind == taskKindDownload || t.meta != "" || t.message != "" || t.status == taskStatusError {
			if w := lipgloss.Width(t.title); w > maxTitleWidth {
				maxTitleWidth = w
			}
		}
	}

	maxDownloadLabelWidth := 0
	if maxTitleWidth > 0 {
		for i := 0; i < shown; i++ {
			t := visibleTasks[i]
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
			task:               visibleTasks[i],
			guide:              guide,
			titleWidth:         maxTitleWidth,
			downloadLabelWidth: maxDownloadLabelWidth,
		}.Line(ctx))
	}
	if len(visibleTasks) > shown {
		lines = append(lines, ctx.styles.clipLine(ctx.width, fmt.Sprintf("  … and %d more", len(visibleTasks)-shown)))
	}

	return lines
}

type ttyTaskComponent struct {
	task  *taskState
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
	case taskStatusRetrying:
		symbol = ctx.styles.taskCanceledIcon.Render("!")
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
	prefix := "  " + guideBar + "  " + symbol + " "
	prefixWidth := lipgloss.Width(prefix)

	content := ""
	switch {
	case t.kind == taskKindDownload:
		content = ttyDownloadContent(t, ctx, c.titleWidth, c.downloadLabelWidth)
	case t.status == taskStatusError:
		if t.meta == "" && t.message != "" {
			title := ttyTaskLabel(t, ctx, c.titleWidth)
			content = title + " " + t.message
		} else {
			titleWidth := 0
			if t.meta != "" {
				titleWidth = c.titleWidth
			}
			title := ttyTaskLabel(t, ctx, titleWidth)
			if t.message != "" {
				content = fmt.Sprintf("%s: %s", title, t.message)
			} else {
				content = title
			}
		}
	case t.status == taskStatusSkipped || t.status == taskStatusCanceled:
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

func ttyTaskLabel(t *taskState, ctx ttyRenderContext, titleWidth int) string {
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

func ttyDownloadLabel(t *taskState, ctx ttyRenderContext, titleWidth int) string {
	if t == nil {
		return ""
	}
	label := ttyTaskLabel(t, ctx, titleWidth)
	if meta := ttyDownloadMeta(t); meta != "" {
		label += " " + ctx.styles.meta.Render(meta)
	}
	return label
}

func ttyDownloadContent(t *taskState, ctx ttyRenderContext, titleWidth, labelWidth int) string {
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
		if t.message != "" {
			parts = append(parts, ctx.styles.message.Render(t.message))
		}
		return strings.Join(parts, "  ")
	case taskStatusRetrying:
		if t.message != "" {
			return label + "  " + ctx.styles.message.Render(t.message)
		}
		return label
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

func ttyDownloadMeta(t *taskState) string {
	if t == nil {
		return ""
	}

	parts := make([]string, 0, 1)
	if t.total > 0 {
		parts = append(parts, fmt.Sprintf("(%s)", formatBytes(t.total)))
	} else if t.status != taskStatusRunning && t.status != taskStatusRetrying && t.current > 0 {
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

func renderTTYBlocks(st *engineState, ctx ttyRenderContext, activeLimit int) [][]string {
	if st == nil {
		return nil
	}
	var blocks [][]string
	for _, g := range st.groups {
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
