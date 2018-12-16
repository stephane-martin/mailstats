package extractors

import (
	"github.com/jordic/goics"
	"github.com/stephane-martin/mailstats/models"
	"time"
)

type IcalConsumer struct {
	Events []*models.Event
}

func (c *IcalConsumer) ConsumeICal(d *goics.Calendar, err error) error {
	if len(d.Events) == 0 {
		return nil
	}
	var zero models.Event
	c.Events = make([]*models.Event, 0, len(d.Events))
	for _, event := range d.Events {
		e := new(models.Event)
		for k, v := range event.Data {
			switch k {
			case "SUMMARY":
				e.Summary = v.Val
			case "DESCRIPTION":
				e.Description = v.Val
			case "DTSTART":
				// "2006-01-02T15:04:05Z07:00"
				t, err := time.Parse("20060102T150405Z", v.Val)
				if err == nil {
					e.Start = &t
				}
			case "DTEND":
				t, err := time.Parse("20060102T150405Z", v.Val)
				if err == nil {
					e.End = &t
				}
			case "LOCATION":
				e.Location = v.Val
			}
		}
		if *e != zero {
			c.Events = append(c.Events, e)
		}

	}
	return nil
}
