package rssreader

import (
	"fmt"
	"time"
)

// Date represents a calendar date (year, month, day) with no time or
// timezone component. It is used for PublishDate on RssItem.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateFromTime extracts the date component from a time.Time value using UTC.
func DateFromTime(t time.Time) Date {
	t = t.UTC()
	return Date{
		Year:  t.Year(),
		Month: t.Month(),
		Day:   t.Day(),
	}
}

// Time returns midnight UTC for this Date, useful for comparisons and sorting.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// String returns the date formatted as YYYY-MM-DD.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// IsZero reports whether the date is the zero value.
func (d Date) IsZero() bool {
	return d.Year == 0 && d.Month == 0 && d.Day == 0
}

// Before reports whether d is earlier than u.
func (d Date) Before(u Date) bool {
	return d.Time().Before(u.Time())
}

// After reports whether d is later than u.
func (d Date) After(u Date) bool {
	return d.Time().After(u.Time())
}

// Equal reports whether d and u represent the same calendar date.
func (d Date) Equal(u Date) bool {
	return d.Year == u.Year && d.Month == u.Month && d.Day == u.Day
}
