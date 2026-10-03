package main

import (
	"database/sql"
	"strconv"
	"time"
)

func itoa(n int) string { return strconv.Itoa(n) }

func f2(x float64) string {
	return strconv.FormatFloat(round2(x), 'f', 2, 64)
}

func fmtTime(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case time.Time:
		return t.Format("2006-01-02 15:04")
	case *time.Time:
		if t == nil {
			return ""
		}
		return t.Format("2006-01-02 15:04")
	case sql.NullTime:
		if !t.Valid {
			return ""
		}
		return t.Time.Format("2006-01-02 15:04")
	}
	return ""
}

func dateOnly(v sql.NullTime) string {
	if !v.Valid {
		return ""
	}
	return v.Time.Format("2006-01-02")
}

type eventJSON struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	PlannedOn string `json:"planned_on"`
	ActualOn  string `json:"actual_on"`
	CreatedAt string `json:"created_at"`
}

func scanEvents(rows *sql.Rows) []eventJSON {
	defer rows.Close()
	list := []eventJSON{}
	for rows.Next() {
		var e eventJSON
		var planned, actual sql.NullTime
		var created time.Time
		if err := rows.Scan(&e.ID, &e.Kind, &e.Title, &e.Detail, &planned, &actual, &created); err != nil {
			continue
		}
		e.PlannedOn = dateOnly(planned)
		e.ActualOn = dateOnly(actual)
		e.CreatedAt = created.Format("2006-01-02 15:04")
		list = append(list, e)
	}
	return list
}
