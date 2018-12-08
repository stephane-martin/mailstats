package models

import "time"

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
	// TODO: check that From is consistent/scam
	From   Address   `json:"from,omitempty"`
	To     []Address `json:"to,omitempty"`
	Title  string    `json:"title,omitempty"`
	Emails []string  `json:"emails,omitempty"`
	Urls   []string  `json:"urls,omitempty"`
	Images []string  `json:"images,omitempty"`
}

type Attachment struct {
	Name         string   `json:"name,omitempty"`
	InferredType string   `json:"inferred_type,omitempty"`
	ReportedType string   `json:"reported_type,omitempty"`
	Size         uint64   `json:"size"`
	Hash         string   `json:"hash,omitempty"`
	PDFMetadata  *PDFMeta `json:"pdf_metadata,omitempty"`
	// TODO: ImageMetadata should be more defined
	ImageMetadata map[string]interface{} `json:"image_metadata,omitempty"`
	// TODO: office document meta
	Archives      map[string]*Archive `json:"archive_content,omitempty"`
	SubAttachment *Attachment         `json:"sub_attachment,omitempty"`
	// TODO
	Executable bool `json:"executable"`
}

type Address struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
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
