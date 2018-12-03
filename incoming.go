package main

import "time"

type BaseInfos struct {
	MailFrom     string    `json:"mail_from,omitempty"`
	RcptTo       []string  `json:"rcpt_to,omitempty"`
	Host         string    `json:"host,omitempty"`
	Family       string    `json:"family,omitempty"`
	Port         int       `json:"port"`
	Addr         string    `json:"addr,omitempty"`
	Helo         string    `json:"helo,omitempty"`
	TimeReported time.Time `json:"timereported,omitempty"`
	UID          [16]byte  `json:"-"`
}

type IncomingMail struct {
	BaseInfos
	Data []byte `json:"data,omitempty"`
}
