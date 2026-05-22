package rssreader

import (
	"encoding/xml"
	"time"
)

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// parseAtom unmarshals XML feed content under the Atom format.
func parseAtom(data []byte, sourceURL string) ([]RssItem, error) {
	var feed atomFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	items := make([]RssItem, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		var publishDate Date
		dateStr := entry.Published
		if dateStr == "" {
			dateStr = entry.Updated
		}
		if dateStr != "" {
			var parsedTime time.Time
			var parsed bool
			for _, layout := range []string{
				time.RFC3339,
				time.RFC3339Nano,
			} {
				if t, err := time.Parse(layout, dateStr); err == nil {
					parsedTime = t
					parsed = true
					break
				}
			}
			if parsed && !parsedTime.IsZero() {
				publishDate = DateFromTime(parsedTime)
			}
		}

		link := ""
		if len(entry.Links) > 0 {
			// Find alternate link or default to first
			for _, l := range entry.Links {
				if l.Rel == "alternate" || l.Rel == "" {
					link = l.Href
					break
				}
			}
			if link == "" {
				link = entry.Links[0].Href
			}
		}

		description := entry.Summary
		if description == "" {
			description = entry.Content
		}

		items = append(items, RssItem{
			Title:       entry.Title,
			Source:      feed.Title,
			SourceURL:   sourceURL,
			Link:        link,
			PublishDate: publishDate,
			Description: description,
		})
	}

	return items, nil
}
