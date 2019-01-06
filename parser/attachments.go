package parser

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"github.com/h2non/filetype/matchers"
	"github.com/inconshreveable/log15"
	"github.com/jordic/goics"
	"github.com/russross/blackfriday"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/xi2/xz"
	"io"
	"io/ioutil"
	"strings"
)

func AnalyseAttachment(filename string, ct string, r io.Reader, t extractors.ExifTool, l log15.Logger) (*models.Attachment, error) {
	content, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	attachment := &models.Attachment{
		Name:         filename,
		Size:         int64(len(content)),
		ReportedType: ct,
	}

	h := sha256.Sum256(content)
	attachment.Hash = hex.EncodeToString(h[:])

	typ, err := utils.Guess(filename, content)
	if err != nil {
		return nil, err
	}
	attachment.InferredType = typ.MIME.Value
	attachment.Executable = extractors.IsExecutable(attachment.InferredType)
	l.Debug("Attachment", "value", typ.MIME.Value, "filename", filename)

	if attachment.Executable {
		meta, err := t.Extract(content, nil, "-EXE:All")
		if err != nil {
			l.Warn("Failed to extract metadata with 'exiftool'", "error", err)
		} else {
			attachment.ExeMetadata = meta
		}
		return attachment, nil
	}

	switch typ {
	case matchers.TypePdf:
		text, err := extractors.PDFBytesToText(content)
		if err != nil {
			l.Warn("Error extracting text from PDF", "error", err)
		} else if len(text) > 0 {
			lang := extractors.Language(text)
			if lang != "" {
				attachment.PDFMetadata = new(models.PDFMeta)
				attachment.PDFMetadata.Language = lang
				attachment.PDFMetadata.Keywords, attachment.PDFMetadata.Phrases = extractors.Keywords(text, nil, attachment.PDFMetadata.Language)
			}
		}
		attachment.PDFMetadata, err = extractors.PDFBytesInfo(content, attachment.PDFMetadata)
		if err != nil {
			l.Warn("Error extracting metadata from PDF", "error", err)
		}
	case matchers.TypeDocx:
		text, props, hasMacro, err := extractors.ConvertBytesDocx(content)
		if err != nil {
			l.Warn("Error extracting metadata from DOCX", "error", err)
		} else {
			attachment.DocMetadata = &models.DocMeta{
				Properties: props,
				HasMacro:   hasMacro,
			}
			if len(text) > 0 {
				lang := extractors.Language(text)
				if lang != "" {
					attachment.DocMetadata.Language = lang
					attachment.DocMetadata.Keywords, attachment.DocMetadata.Phrases = extractors.Keywords(text, nil, attachment.DocMetadata.Language)
				}
			}
		}
	case utils.OdtType:
		text, props, err := extractors.ConvertBytesODT(content)
		if err != nil {
			l.Warn("Error extracting metadata from ODT", "error", err)
		} else {
			attachment.DocMetadata = new(models.DocMeta)
			attachment.DocMetadata.Properties = props
			if len(text) > 0 {
				lang := extractors.Language(text)
				if lang != "" {
					attachment.DocMetadata.Language = lang
					attachment.DocMetadata.Keywords, attachment.DocMetadata.Phrases = extractors.Keywords(text, nil, attachment.DocMetadata.Language)
				}
			}
		}

	case matchers.TypeDoc:
		text, err := extractors.ConvertBytesDoc(content)
		if err != nil {
			l.Warn("Error extracting text from DOC", "error", err)
		} else {
			if len(text) > 0 {
				lang := extractors.Language(text)
				if lang != "" {
					attachment.DocMetadata = new(models.DocMeta)
					attachment.DocMetadata.Language = lang
					attachment.DocMetadata.Keywords, attachment.DocMetadata.Phrases = extractors.Keywords(text, nil, attachment.DocMetadata.Language)
				}
			}
			p, err := t.Extract(content, nil, "-FlashPix:All")
			if err != nil {
				l.Warn("Error extracting metadata from DOC", "error", err)
			} else {
				if attachment.DocMetadata == nil {
					attachment.DocMetadata = new(models.DocMeta)
				}
				attachment.DocMetadata.Properties = p
			}
		}

	case utils.HTMLType:
		text, _, _ := extractors.HTML2Text(string(content))
		if len(text) > 0 {
			lang := extractors.Language(text)
			if lang != "" {
				attachment.DocMetadata = new(models.DocMeta)
				attachment.DocMetadata.Language = lang
				attachment.DocMetadata.Keywords, attachment.DocMetadata.Phrases = extractors.Keywords(text, nil, attachment.DocMetadata.Language)
			}
		}

	case utils.MarkdownType:
		html := string(blackfriday.Run(content))
		text, _, _ := extractors.HTML2Text(html)
		if len(text) > 0 {
			lang := extractors.Language(text)
			if lang != "" {
				attachment.DocMetadata = new(models.DocMeta)
				attachment.DocMetadata.Language = lang
				attachment.DocMetadata.Keywords, attachment.DocMetadata.Phrases = extractors.Keywords(text, nil, attachment.DocMetadata.Language)
			}
		}

	case matchers.TypePng:
		if t != nil {
			meta, err := t.Extract(content, nil, "-EXIF:All", "-PNG:All")
			if err != nil {
				l.Warn("Failed to extract metadata with exiftool", "error", err)
			} else {
				attachment.ImageMetadata = meta
			}
		}

	case matchers.TypeJpeg, matchers.TypeWebp, matchers.TypeGif:
		if t != nil {
			meta, err := t.Extract(content, nil, "-EXIF:All")
			if err != nil {
				l.Warn("Failed to extract metadata with exiftool", "error", err)
			} else {
				attachment.ImageMetadata = meta
			}
		}
	case matchers.TypeZip, matchers.TypeTar, matchers.TypeRar:
		archive, err := AnalyzeArchive(typ, bytes.NewReader(content), attachment.Size, l)
		if err != nil {
			l.Warn("Error analyzing archive", "error", err)
		} else {
			if attachment.Archives == nil {
				attachment.Archives = make(map[string]*models.Archive)
			}
			attachment.Archives[filename] = archive
		}

	case matchers.TypeGz:
		gzReader, err := gzip.NewReader(bytes.NewReader(content))
		if err != nil {
			l.Warn("Failed to decompress gzip attachement", "error", err)
		} else {
			if strings.HasSuffix(filename, ".gz") {
				filename = filename[:len(filename)-3]
			}
			subAttachment, err := AnalyseAttachment(filename, "", gzReader, t, l)
			if err != nil {
				l.Warn("Error analyzing subattachement", "error", err)
			} else {
				attachment.SubAttachment = subAttachment
				attachment.Executable = subAttachment.Executable
			}
		}

	case matchers.TypeBz2:
		bz2Reader := bzip2.NewReader(bytes.NewReader(content))
		if strings.HasSuffix(filename, ".bz2") {
			filename = filename[:len(filename)-4]
		}
		subAttachment, err := AnalyseAttachment(filename, "", bz2Reader, t, l)
		if err != nil {
			l.Warn("Error analyzing sub-attachement", "error", err)
		} else {
			attachment.SubAttachment = subAttachment
			attachment.Executable = subAttachment.Executable
		}

	case matchers.TypeXz:
		xzReader, err := xz.NewReader(bytes.NewReader(content), 0)
		if err != nil {
			l.Warn("Failed to decompress xz attachement", "error", err)
		} else {
			if strings.HasSuffix(filename, ".xz") {
				filename = filename[:len(filename)-3]
			}
			subAttachment, err := AnalyseAttachment(filename, "", xzReader, t, l)
			if err != nil {
				l.Warn("Error analyzing sub-attachment", "error", err)
			} else {
				attachment.SubAttachment = subAttachment
				attachment.Executable = subAttachment.Executable
			}
		}

	case utils.IcalType:
		dec := goics.NewDecoder(bytes.NewReader(content))
		c := new(extractors.IcalConsumer)
		err := dec.Decode(c)
		if err != nil {
			l.Warn("Failed to decode iCal", "error", err)
		} else if c.Events != nil {
			attachment.EventsMetadata = c.Events
		}

	default:
		l.Info("Unknown attachment type", "type", typ.MIME.Value)
	}

	return attachment, nil
}
