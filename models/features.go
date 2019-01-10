package models

import (
	"encoding/json"
	"net"
	"time"
)

type FeaturesMail struct {
	BaseInfos   `yaml:",inline"`
	UID         string              `json:"uid,omitempty"`
	Size        int64               `json:"size_bytes"`
	ContentType string              `json:"content_type,omitempty"`
	Reported    string              `json:"timereported,omitempty"`
	Headers     map[string][]string `json:"headers,omitempty"`
	Attachments []*Attachment       `json:"attachments,omitempty"`
	BagOfWords  map[string]int      `json:"bag_of_words,omitempty"`
	Keywords    []string            `json:"keywords,omitempty" yaml:",flow"`
	Phrases     []string            `json:"phrases,omitempty" yaml:",flow"`
	Language    string              `json:"language,omitempty"`
	TimeHeader  string              `json:"time_header,omitempty"`
	Received    []ReceivedElement   `json:"received,omitempty"`
	// TODO: check that From is consistent/scam
	From          *FromAddress      `json:"from,omitempty"`
	To            []Address         `json:"to,omitempty" yaml:",flow"`
	Title         string            `json:"title,omitempty"`
	Emails        []string          `json:"emails,omitempty"`
	URLs          []string          `json:"urls,omitempty"`
	PhishtankURLS []*PhishtankEntry `json:"phishtank_urls,omitempty"`
	Images        []string          `json:"images,omitempty"`
	DKIM          *DKIMValidation   `json:"dkim,omitempty"`
}

func (f *FeaturesMail) Encode(indent bool) ([]byte, error) {
	if indent {
		return json.MarshalIndent(f, "", "  ")
	} else {
		return json.Marshal(f)
	}
}

type DKIMValidation struct {
	Error         string             `json:"error,omitempty"`
	Verifications []DKIMVerification `json:"verifications,omitempty"`
}

type DKIMVerification struct {
	Domain     string `json:"domain,omitempty"`
	Identifier string `json:"identifier,omitempty"`
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
	Raw          string        `json:"raw,omitempty"`
}

type ReceivedFrom struct {
	ReportedName    string      `json:"reported_name,omitempty"`
	IP              *ReceivedIP `json:"ip,omitempty"`
	DNSReversedName string      `json:"reversed_name,omitempty"`
}

type ReceivedIP struct {
	Parsed   net.IP       `json:"parsed_ip,omitempty"`
	Reported string       `json:"reported,omitempty"`
	Public   bool         `json:"public"`
	Location *GeoIPResult `json:"location,omitempty"`
}

type ReceivedBy struct {
	Name     string `json:"name,omitempty"`
	Software string `json:"software,omitempty"`
}

type GeoIPResult struct {
	Country     string  `json:"country,omitempty"`
	Continent   string  `json:"continent,omitempty"`
	City        string  `json:"city,omitempty"`
	Coordinates *LatLon `json:"coordinates,omitempty"`
}

type LatLon struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

type PhishtankEntry struct {
	URL              string     `json:"url,omitempty"`
	PhishID          int        `json:"phish_id"`
	DetailsURL       string     `json:"details_url,omitempty"`
	SubmissionTime   *time.Time `json:"submission_time,omitempty"`
	Online           bool       `json:"online"`
	Verified         bool       `json:"verified"`
	VerificationTime *time.Time `json:"verification_time,omitempty"`
}

type Attachment struct {
	Name           string                 `json:"name,omitempty"`
	InferredType   string                 `json:"inferred_type,omitempty"`
	ReportedType   string                 `json:"reported_type,omitempty"`
	Size           int64                  `json:"size_bytes"`
	Hash           string                 `json:"hash,omitempty"`
	PDFMetadata    *PDFMeta               `json:"pdf_metadata,omitempty"`
	DocMetadata    *DocMeta               `json:"doc_metadata,omitempty"`
	ExeMetadata    map[string]interface{} `json:"exe_metadata,omitempty"`
	EventsMetadata []*Event               `json:"event_metadata,omitempty"`
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

type FromAddress struct {
	Address
	Error               string `json:"error,omitempty"`
	NameContainsAddress bool   `json:"name_contains_address"`
	Multiple            bool   `json:"multiple"`
	Different           bool   `json:"different"`
	Spoofed             bool   `json:"spoofed"`
}

type Event struct {
	Summary     string     `json:"summary,omitempty"`
	Description string     `json:"description,omitempty"`
	Location    string     `json:"location,omitempty"`
	Start       *time.Time `json:"start,omitempty"`
	End         *time.Time `json:"end,omitempty"`
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
	Pages          int64      `json:"pages"`
	Encrypted      bool       `json:"encrypted"`
	PageSize       string     `json:"page_size,omitempty"`
	FileSize       int64      `json:"file_size_bytes"`
	Optimized      bool       `json:"optimized"`
	Version        string     `json:"version,omitempty"`
	Language       string     `json:"language,omitempty"`
	Keywords       []string   `json:"keywords,omitempty"`
	Phrases        []string   `json:"phrases,omitempty"`
}

type DocMeta struct {
	HasMacro   bool                   `json:"has_macro"` // TODO
	Language   string                 `json:"language,omitempty"`
	Keywords   []string               `json:"keywords,omitempty"`
	Phrases    []string               `json:"phrases,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type ArchiveFile struct {
	Name        string `json:"name,omitempty"`
	Extension   string `json:"extension,omitempty"`
	Type        string `json:"type,omitempty"`
	Compression string `json:"compression,omitempty"`
}

type Archive struct {
	Files              []*ArchiveFile      `json:"files,omitempty"`
	DecompressedSize   int64               `json:"decompressed_size_bytes"`
	ArchiveType        string              `json:"type,omitempty"`
	SubArchives        map[string]*Archive `json:"sub_archives,omitempty"`
	ContainsExecutable bool                `json:"contains_exe"`
}
