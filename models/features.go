package models

import (
	"net"
	"time"
)

type FeaturesMail struct {
	BaseInfos
	UID          string              `json:"uid,omitempty"`
	Size         int                 `json:"size"`
	ContentType  string              `json:"content_type,omitempty"`
	TimeReported string              `json:"timereported,omitempty"`
	Headers      map[string][]string `json:"headers,omitempty"`
	Attachments  []*Attachment       `json:"attachments,omitempty"`
	BagOfWords   map[string]int      `json:"bag_of_words,omitempty"`
	BagOfStems   map[string]int      `json:"bag_of_stems,omitempty"`
	Keywords     []string            `json:"keywords,omitempty"`
	Language     string              `json:"language,omitempty"`
	TimeHeader   string              `json:"time_header,omitempty"`
	Received     []ReceivedElement   `json:"received,omitempty"`
	// TODO: check that From is consistent/scam
	From   Address   `json:"from,omitempty"`
	To     []Address `json:"to,omitempty"`
	Title  string    `json:"title,omitempty"`
	Emails []string  `json:"emails,omitempty"`
	Urls   []string  `json:"urls,omitempty"`
	Images []string  `json:"images,omitempty"`
}

type ReceivedElement struct {
	From         *ReceivedFrom `json:"from,omitempty"`
	By           *ReceivedBy   `json:"by,omitempty"`
	Protocol     string        `json:"protocol,omitempty"`
	ID           string        `json:"id,omitempty"`
	TLS          string        `json:"tls,omitempty"`
	For          string        `json:"for,omitempty"`
	Timestamp    *time.Time    `json:"time,omitempty"`
	EnvelopeFrom string        `json:"envelope_from,omitempty"`
	Helo         string        `json:"helo,omitempty"`
	Raw          string        `json:"raw"`
}

type ReceivedFrom struct {
	ReportedName    string      `json:"reported_name,omitempty"`
	IP              *ReceivedIP `json:"ip,omitempty"`
	DNSReversedName string      `json:"reversed_name,omitempty"`
}

type ReceivedIP struct {
	Parsed   net.IP       `json:"parsed,omitempty"`
	Reported string       `json:"reported,omitempty"`
	Public   bool         `json:"public"`
	Location *GeoIPResult `json:"location,omitempty"`
}

type ReceivedBy struct {
	Name     string `json:"name,omitempty"`
	Software string `json:"software,omitempty"`
}

type GeoIPResult struct {
	Country   string  `json:"country,omitempty"`
	Continent string  `json:"continent,omitempty"`
	City      string  `json:"city,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type Attachment struct {
	Name           string   `json:"name,omitempty"`
	InferredType   string   `json:"inferred_type,omitempty"`
	ReportedType   string   `json:"reported_type,omitempty"`
	Size           uint64   `json:"size"`
	Hash           string   `json:"hash,omitempty"`
	PDFMetadata    *PDFMeta `json:"pdf_metadata,omitempty"`
	DocMetadata    *DocMeta `json:"doc_metadata,omitempty"`
	EventsMetadata []*Event `json:"event_metadata,omitempty"`
	// TODO: ImageMetadata should be more defined
	ImageMetadata map[string]interface{} `json:"image_metadata,omitempty"`
	Archives      map[string]*Archive    `json:"archive_content,omitempty"`
	SubAttachment *Attachment            `json:"sub_attachment,omitempty"`
	// TODO
	Executable bool `json:"is_executable"`
}

type Address struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
}

type Event struct {
	Summary     string
	Description string
	Location    string
	Start       *time.Time
	End         *time.Time
}

type PDFMeta struct {
	Title          string     `json:"title,omitempty"`
	Author         string     `json:"author,omitempty"`
	Creator        string     `json:"creator,omitempty"`
	Producer       string     `json:"producer,omitempty"`
	CreationDate   *time.Time `json:"creation_date,omitempty"`
	ModDate        *time.Time `json:"mod_date,omitempty"`
	Tagged         bool       `json:"tagged"`
	UserProperties bool       `json:"user_properties"`
	Form           string     `json:"form,omitempty"`
	Javascript     bool       `json:"javascript"`
	Pages          int        `json:"pages"`
	Encrypted      bool       `json:"encrypted"`
	PageSize       string     `json:"page_size,omitempty"`
	FileSize       int        `json:"file_size"`
	Optimized      bool       `json:"optimized"`
	Version        string     `json:"version,omitempty"`
}

type DocMeta struct {
	HasMacro   bool                   `json:"has_macro"`
	Language   string                 `json:"language"`
	Keywords   []string               `json:"keywords"`
	Properties map[string]interface{} `json:"properties"`
}

type ArchiveFile struct {
	Name        string `json:"name,omitempty"`
	Extension   string `json:"extension,omitempty"`
	Type        string `json:"type,omitempty"`
	Compression string `json:"compression,omitempty"`
}

type Archive struct {
	Files            []*ArchiveFile      `json:"files,omitempty"`
	DecompressedSize uint64              `json:"decompressed_size"`
	ArchiveType      string              `json:"type,omitempty"`
	SubArchives      map[string]*Archive `json:"sub_archives,omitempty"`
	// TODO
	ContainsExecutable bool `json:"contains_exe"`
}
