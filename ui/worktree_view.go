package ui

import "strings"

type WorktreeRow struct {
	BranchLabel   string
	PRLabel       string
	CILabel       string
	ReviewLabel   string
	PRStatusLabel string
	Disabled      bool
}

func RenderWorktreeSelector(rows []WorktreeRow, cursor int, styles Styles) string {
	const (
		branchWidth   = 40
		prWidth       = 12
		ciWidth       = 12
		approvedWidth = 12
		prStateWidth  = 10
	)
	var b strings.Builder
	header := formatWorktreeLine("Branch", "PR", "CI", "Approved", "PR Status", branchWidth, prWidth, ciWidth, approvedWidth, prStateWidth)
	b.WriteString(styles.Header("  " + header))
	b.WriteString("\n")
	for i, row := range rows {
		rowStyle := styles.Normal
		rowSelectedStyle := styles.Selected
		if row.Disabled {
			rowStyle = styles.Disabled
			rowSelectedStyle = styles.DisabledSelected
		}
		line := formatWorktreeLine(
			row.BranchLabel,
			row.PRLabel,
			row.CILabel,
			row.ReviewLabel,
			row.PRStatusLabel,
			branchWidth,
			prWidth,
			ciWidth,
			approvedWidth,
			prStateWidth,
		)
		if i == cursor {
			b.WriteString("  " + rowSelectedStyle(line))
		} else {
			b.WriteString("  " + rowStyle(line))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatWorktreeLine(branch string, pr string, ci string, approved string, prState string, branchWidth int, prWidth int, ciWidth int, approvedWidth int, prStateWidth int) string {
	return PadOrTrim(branch, branchWidth) + " " +
		PadOrTrim(pr, prWidth) + " " +
		PadOrTrim(ci, ciWidth) + " " +
		PadOrTrim(approved, approvedWidth) + " " +
		PadOrTrim(prState, prStateWidth)
}
