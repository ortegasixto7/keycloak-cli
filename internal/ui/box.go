package ui

import "strings"

type BoxOptions struct {
	JiraTicket string
	Realm      string
	Title      string
}

func RenderBox(lines []string, opts BoxOptions) string {
	headerText := buildHeaderText(opts)
	contentWidth := len(headerText)
	for _, l := range lines {
		if len(l) > contentWidth {
			contentWidth = len(l)
		}
	}
	if contentWidth < 80 {
		contentWidth = 80
	}
	topBottom := "|" + strings.Repeat(":", contentWidth+2) + "|"

	var b strings.Builder
	b.WriteString(topBottom)
	b.WriteString("\n")

	headerPadded := padRight(headerText, contentWidth)
	b.WriteString("| " + headerPadded + " |\n")

	for _, l := range lines {
		padded := padRight(l, contentWidth)
		b.WriteString("| " + padded + " |\n")
	}

	b.WriteString(topBottom)
	return b.String()
}

func buildHeaderText(opts BoxOptions) string {
	parts := make([]string, 0, 3)
	if opts.JiraTicket != "" {
		parts = append(parts, "Jira Ticket: "+opts.JiraTicket)
	}
	if opts.Realm != "" {
		parts = append(parts, "Current realm: "+opts.Realm)
	}
	if len(parts) == 0 {
		if opts.Title != "" {
			return opts.Title
		}
		return "Keycloak CLI"
	}
	return strings.Join(parts, " ::: ")
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
