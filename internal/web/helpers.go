package web

import (
	"time"
)

func FormatDate(t time.Time) string {
	return t.Format("Jan 2, 2006")
}

func FormatDateTime(t time.Time) string {
	return t.Format("Jan 2, 2006 15:04")
}

func FormatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return formatPlural(minutes, "minute")
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return formatPlural(hours, "hour")
	}
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return formatPlural(days, "day")
	}
	if diff < 30*24*time.Hour {
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return formatPlural(weeks, "week")
	}
	if diff < 365*24*time.Hour {
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return formatPlural(months, "month")
	}
	years := int(diff.Hours() / 24 / 365)
	if years == 1 {
		return "1 year ago"
	}
	return formatPlural(years, "year")
}

func formatPlural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return formatInt(n) + " " + unit + "s ago"
}

func formatInt(n int) string {
	return time.Duration(n).String()
}
