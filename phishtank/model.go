package phishtank

import (
	"github.com/stephane-martin/mailstats/models"
	"net/url"
	"strings"
	"time"
)

type Entry struct {
	URL          string       `xml:"url"`
	PhishID      int          `xml:"phish_id"`
	DetailURL    string       `xml:"phish_detail_url"`
	Details      Details      `xml:"details"`
	Submission   Submission   `xml:"submission"`
	Verification Verification `xml:"verification"`
	Status       Status       `xml:"status"`
	Target       string       `xml:"target"`
}

func (e *Entry) Parse() *models.PhishtankEntry {
	if e == nil {
		return nil
	}
	u, err := url.Parse(e.URL)
	if err != nil {
		return nil
	}
	v, err := url.PathUnescape(u.String())
	if err != nil {
		return nil
	}
	gEntry := &models.PhishtankEntry{
		URL:        strings.Replace(v, "&amp;", "&", -1),
		PhishID:    e.PhishID,
		DetailsURL: e.DetailURL,
		Online:     strings.ToLower(strings.TrimSpace(e.Status.Online)) == "yes",
		Verified:   strings.ToLower(strings.TrimSpace(e.Verification.Verified)) == "yes",
	}
	vt, err := time.Parse(time.RFC3339, e.Verification.Time)
	if err == nil {
		gEntry.VerificationTime = &vt
	}
	st, err := time.Parse(time.RFC3339, e.Submission.Time)
	if err == nil {
		gEntry.SubmissionTime = &st
	}
	return gEntry
}

type Details struct {
	Details []Detail `xml:"detail"`
}

type Detail struct {
	IPAddress         string `xml:"ip_address"`
	CIDRBlock         string `xml:"cidr_block"`
	AnnouncingNetwork string `xml:"announcing_network"`
	RIR               string `xml:"rir"`
	Time              string `xml:"detail_time"`
}

type Submission struct {
	Time string `xml:"submission_time"`
}

type Verification struct {
	Verified string `xml:"verified"`
	Time     string `xml:"verification_time"`
}

type Status struct {
	Online string `xml:"online"`
}
