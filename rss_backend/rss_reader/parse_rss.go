package rssreader

import (
	"encoding/xml"
	"time"
)

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

// parseRSS unmarshals XML feed content under the RSS 2.0 format.
func parseRSS(data []byte, sourceURL string) ([]RssItem, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	items := make([]RssItem, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		var publishDate Date
		if item.PubDate != "" {
			var parsedTime time.Time
			var parsed bool
			for _, layout := range []string{
				time.RFC1123Z,
				time.RFC1123,
				"Mon, _2 Jan 2006 15:04:05 MST",
				"Mon, _2 Jan 2006 15:04:05 -0700",
			} {
				if t, err := time.Parse(layout, item.PubDate); err == nil {
					parsedTime = t
					parsed = true
					break
				}
			}
			if parsed && !parsedTime.IsZero() {
				publishDate = DateFromTime(parsedTime)
			}
		}

		items = append(items, RssItem{
			Title:       item.Title,
			Source:      feed.Channel.Title,
			SourceURL:   sourceURL,
			Link:        item.Link,
			PublishDate: publishDate,
			Description: item.Description,
		})
	}

	return items, nil
}
