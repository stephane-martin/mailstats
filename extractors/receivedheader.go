package extractors

import (
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"net"
	"regexp"
	"strings"
	"time"
)

// from m0892.contabo.net ([91.194.91.211]:40338 helo=91.194.91.211) by m1363.contabo.net with esmtpsa (TLSv1.2:ECDHE-RSA-AES256-GCM-SHA384:256) (Exim 4.88) (envelope-from <support@contabo.com>) id 1ciAQt-0005YX-7L; Mon, 27 Feb 2017 02:48:59 +0100
// (envelope-from <newsletter@liberation.cccampaigns.net>)
// (envelope-from <bounce-md_30850198.5c14c57c.v1-a48f024874c347249d7f5eb681ffab1d@mandrillapp.com>)


var receivedHeaderRE = regexp.MustCompile(`(from ([^\s();]+)\s+(\(\s*(([^\s\[]+)?(\s*\[\s*(\S+)\s*][:\d]*\s*)?)\)\s+)?)?by\s+([^\s;]+)(\s+\(([^;)]+\)*)\))?(\s+with\s+([^\s;]+))?(\s+id\s+([^\s;]+))?(\s+\(([^)]+)\))?(\s+for\s+([^\s;]+))?;?(.*?)$`)
var envelopeRE = regexp.MustCompile(`\(envelope-from (\S+)\)`)
var heloRE = regexp.MustCompile(`helo=([^\s()[\]]+)`)

func ParseReceivedHeader(h string, logger log15.Logger) (e models.ReceivedElement) {
	e.Raw = h
	envMatches := envelopeRE.FindStringSubmatch(h)
	if len(envMatches) == 2 {
		e.EnvelopeFrom = strings.TrimSpace(envMatches[1])
		h = envelopeRE.ReplaceAllString(h, "")
	}
	heloMatches := heloRE.FindStringSubmatch(h)
	if len(heloMatches) == 2 {
		e.Helo = strings.TrimSpace(heloMatches[1])
		h = heloRE.ReplaceAllString(h, "")
	}
	matches := receivedHeaderRE.FindStringSubmatch(h)
	if len(matches) == 0 {
		return e
	}
	if len(matches) > 2 && matches[2] != "" {
		e.From = new(models.ReceivedFrom)
		e.From.ReportedName = strings.Trim(matches[2], "[]")
		if net.ParseIP(e.From.ReportedName) != nil {
			e.From.IP = &models.ReceivedIP{Reported: e.From.ReportedName}
			if e.Helo != "" {
				e.From.ReportedName = e.Helo
			}
		}
	}

	if len(matches) > 5 && matches[5] != "" {
		if e.From == nil {
			e.From = new(models.ReceivedFrom)
		}
		e.From.DNSReversedName = matches[5]
	}
	if len(matches) > 7 && matches[7] != "" {
		if e.From == nil {
			e.From = new(models.ReceivedFrom)
		}
		if e.From.IP == nil {
			e.From.IP = &models.ReceivedIP{Reported: matches[7]}
		}
	}
	if e.From != nil && e.From.IP != nil && e.From.IP.Reported != "" {
		if ip := net.ParseIP(e.From.IP.Reported); ip != nil {
			e.From.IP.Parsed = ip
			e.From.IP.Public = !utils.IsPrivateIP(e.From.IP.Parsed)
			if e.From.IP.Public && utils.GeoIPEnabled() {
				l, err := utils.GeoIP(e.From.IP.Parsed)
				if err == nil {
					e.From.IP.Location = l
				}
			}
		}
	}

	if len(matches) > 8 && matches[8] != "" {
		e.By = new(models.ReceivedBy)
		e.By.Name = matches[8]
	}
	if len(matches) > 10 && matches[10] != "" {
		if e.By == nil {
			e.By = new(models.ReceivedBy)
		}
		e.By.Software = matches[10]
	}
	if len(matches) > 12 && matches[12] != "" {
		e.Protocol = matches[12]
	}
	if len(matches) > 14 && matches[14] != "" {
		e.ID = matches[14]
	}
	if len(matches) > 16 && matches[16] != "" {
		e.TLS = matches[16]
	}
	if len(matches) > 18 && matches[18] != "" {
		e.For = matches[18]
	}
	if len(matches) > 19 {
		e.Timestamp = parseDate(matches[19], logger)
	}
	return e
}

func parseDate(d string, logger log15.Logger) *time.Time {
	d = strings.TrimSpace(d)
	if len(d) == 0 {
		return nil
	}
	if idx := strings.Index(d, " m="); idx != -1 {
		d = strings.TrimSpace(d[:idx])
	}
	if idx := strings.Index(d, ";"); idx != -1 {
		d = strings.TrimSpace(d[idx+1:])
	}
	for _, format := range timeFormats {
		t, err := time.Parse(format, d)
		if err == nil {
			return &t
		}
	}
	logger.Info("Failed to parse timestamp", "timestamp", d)

	return nil
}

// Thu,  6 Dec 2018 12:20:17 +0100 (CET)
// Thu, 6 Dec 2018 12:20:11 +0100 (CET)
// Thu, 6 Dec 2018 11:20:06 +0000

var timeFormats = []string{
	"Mon, 02 Jan 2006 15:04:05 MST",
	"Mon, 02 Jan 2006 15:04:05 -0700",
	"Mon, _2 Jan 2006 15:04:05 -0700",
	"Mon,_2 Jan 2006 15:04:05 -0700",
	"Mon, 02 Jan 2006 15:04:05 MST",
	"Mon, _2 Jan 2006 15:04:05 MST",
	"Mon,_2 Jan 2006 15:04:05 MST",
	"Mon, 02 Jan 2006 15:04:05 (MST)",
	"Mon, _2 Jan 2006 15:04:05 (MST)",
	"Mon,_2 Jan 2006 15:04:05 (MST)",
	"Mon, 02 Jan 2006 15:04:05 -0700 MST",
	"Mon, _2 Jan 2006 15:04:05 -0700 MST",
	"Mon,_2 Jan 2006 15:04:05 -0700 MST",
	"Mon, 02 Jan 2006 15:04:05 -0700 (MST)",
	"Mon, _2 Jan 2006 15:04:05 -0700 (MST)",
	"Mon,_2 Jan 2006 15:04:05 -0700 (MST)",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05 (MST)",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05 -0700 MST",
	"2006-01-02 15:04:05 -0700 (MST)",
}

