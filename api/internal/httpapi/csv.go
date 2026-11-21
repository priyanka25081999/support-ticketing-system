package httpapi

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"ticketing/api/internal/domain"
)

const csvTimeLayout = "2006-01-02 15:04:05 MST"

// csvSafe prevents CSV/formula injection when the file is opened in Excel/Sheets.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func writeTicketsCSV(w io.Writer, tickets []domain.Ticket, loc *time.Location) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Ticket", "Subject", "Type", "Severity", "Status", "Requester", "Assignee",
		"Created", "Updated", "Solved", "Resolution hours"})
	for _, t := range tickets {
		assignee, solved, hours := "", "", ""
		if t.Assignee != nil {
			assignee = t.Assignee.Name
		}
		if t.SolvedAt != nil {
			solved = t.SolvedAt.In(loc).Format(csvTimeLayout)
			hours = fmt.Sprintf("%.1f", t.SolvedAt.Sub(t.CreatedAt).Hours())
		}
		_ = cw.Write([]string{
			t.TicketNumber, csvSafe(t.Subject), t.Type, t.Severity, string(t.Status),
			csvSafe(t.Requester.Name), csvSafe(assignee),
			t.CreatedAt.In(loc).Format(csvTimeLayout), t.UpdatedAt.In(loc).Format(csvTimeLayout), solved, hours,
		})
	}
	cw.Flush()
	return cw.Error()
}
