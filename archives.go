package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"github.com/nwaples/rardecode"
	"github.com/xi2/xz"
	"io"
	"io/ioutil"
	"path/filepath"
)

type ArchiveFile struct {
	Name        string `json:"name,omitempty"`
	Extension   string `json:"extension,omitempty"`
	Type        string `json:"type,omitempty"`
	Compression string `json:"compression,omitempty"`
}

type Archive struct {
	Files              []*ArchiveFile      `json:"files,omitempty"`
	DecompressedSize   uint64              `json:"decompressed_size"`
	ArchiveType        string              `json:"type,omitempty"`
	SubArchives        map[string]*Archive `json:"sub_archives,omitempty"`
	ContainsExecutable bool                `json:"contains_exe"`
}

func AnalyzeArchive(typ types.Type, reader *bytes.Reader, size uint64) (*Archive, error) {
	switch typ {
	case matchers.TypeZip:
		return AnalyzeZip(reader, size)
	case matchers.TypeTar:
		return AnalyzeTar(reader)
	case matchers.TypeRar:
		return AnalyzeRar(reader)
	default:
		return nil, errors.New("unknown archive type")
	}
}

func replaceCompressed(oldType types.Type, oldReader io.Reader) (newType types.Type, newReader io.Reader, compression string) {
	newReader = oldReader
	newType = oldType

	switch oldType {
	case matchers.TypeGz:
		compression = "gzip"
		r, err := gzip.NewReader(oldReader)
		if err == nil {
			newt, newr, err := Guess(r)
			if err == nil {
				newType = newt
				newReader = newr
			}
		}
	case matchers.TypeBz2:
		compression = "bzip2"
		r := bzip2.NewReader(oldReader)
		newt, newr, err := Guess(r)
		if err == nil {
			newType = newt
			newReader = newr
		}
	case matchers.TypeXz:
		compression = "xz"
		r, err := xz.NewReader(oldReader, 0)
		if err == nil {
			newt, newr, err := Guess(r)
			if err == nil {
				newType = newt
				newReader = newr
			}
		}
	default:
	}
	return
}

func AnalyzeZip(reader io.ReaderAt, size uint64) (*Archive, error) {
	zipReader, err := zip.NewReader(reader, int64(size))
	if err != nil {
		return nil, err
	}
	archive := new(Archive)
	archive.ArchiveType = "zip"

LoopFiles:
	for _, f := range zipReader.File {
		entry := ArchiveFile{Name: f.Name, Extension: filepath.Ext(f.Name)}
		archive.Files = append(archive.Files, &entry)
		archive.DecompressedSize += f.UncompressedSize64
		if f.FileInfo().IsDir() {
			continue LoopFiles
		}

		fileReader, err := f.Open()
		if err == nil {
			t, newReader, err := Guess(fileReader)
			if err == nil {
				t, newReader, entry.Compression = replaceCompressed(t, newReader)
				entry.Type = t.MIME.Value
				switch t {
				case matchers.TypeTar:
					subArchive, err := AnalyzeTar(newReader)
					if err == nil {
						if archive.SubArchives == nil {
							archive.SubArchives = make(map[string]*Archive)
						}
						archive.SubArchives[f.Name] = subArchive
					}
				case matchers.TypeRar:
					subArchive, err := AnalyzeRar(newReader)
					if err == nil {
						if archive.SubArchives == nil {
							archive.SubArchives = make(map[string]*Archive)
						}
						archive.SubArchives[f.Name] = subArchive
					}
				case matchers.TypeZip:
					content, err := ioutil.ReadAll(newReader)
					if err == nil {
						subArchive, err := AnalyzeZip(bytes.NewReader(content), uint64(len(content)))
						if err == nil {
							if archive.SubArchives == nil {
								archive.SubArchives = make(map[string]*Archive)
							}
							archive.SubArchives[f.Name] = subArchive
						}
					}
				}
			}
			_ = fileReader.Close()
		}
	}
	return archive, nil

}

func AnalyzeRar(reader io.Reader) (*Archive, error) {
	rarReader, err := rardecode.NewReader(reader, "")
	if err != nil {
		return nil, err
	}
	archive := new(Archive)
	archive.ArchiveType = "rar"
LoopFiles:
	for {
		header, err := rarReader.Next()
		if err == io.EOF {
			return archive, nil
		}
		if err != nil {
			return archive, err
		}
		entry := ArchiveFile{Name: header.Name, Extension: filepath.Ext(header.Name)}
		archive.Files = append(archive.Files, &entry)
		if !header.UnKnownSize {
			archive.DecompressedSize += uint64(header.UnPackedSize)
		}
		if header.IsDir {
			continue LoopFiles
		}

		t, newReader, err := Guess(rarReader)
		if err == nil {
			t, newReader, entry.Compression = replaceCompressed(t, newReader)
			entry.Type = t.MIME.Value
			switch t {
			case matchers.TypeTar:
				subArchive, err := AnalyzeTar(newReader)
				if err == nil {
					if archive.SubArchives == nil {
						archive.SubArchives = make(map[string]*Archive)
					}
					archive.SubArchives[header.Name] = subArchive
				}
			case matchers.TypeRar:
				subArchive, err := AnalyzeRar(newReader)
				if err == nil {
					if archive.SubArchives == nil {
						archive.SubArchives = make(map[string]*Archive)
					}
					archive.SubArchives[header.Name] = subArchive
				}
			case matchers.TypeZip:
				content, err := ioutil.ReadAll(newReader)
				if err == nil {
					subArchive, err := AnalyzeZip(bytes.NewReader(content), uint64(len(content)))
					if err == nil {
						if archive.SubArchives == nil {
							archive.SubArchives = make(map[string]*Archive)
						}
						archive.SubArchives[header.Name] = subArchive
					}
				}
			}
		}
	}
}

func AnalyzeTar(reader io.Reader) (*Archive, error) {
	tarReader := tar.NewReader(reader)
	archive := new(Archive)
	archive.ArchiveType = "tar"
LoopFiles:
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return archive, nil
		}
		if err != nil {
			return archive, err
		}
		entry := ArchiveFile{Name: header.Name, Extension: filepath.Ext(header.Name)}
		archive.Files = append(archive.Files, &entry)
		archive.DecompressedSize += uint64(header.Size)
		if header.Typeflag != tar.TypeReg {
			continue LoopFiles
		}
		t, newReader, err := Guess(tarReader)
		if err == nil {
			t, newReader, entry.Compression = replaceCompressed(t, newReader)
			entry.Type = t.MIME.Value
			switch t {
			case matchers.TypeTar:
				subArchive, err := AnalyzeTar(newReader)
				if err == nil {
					if archive.SubArchives == nil {
						archive.SubArchives = make(map[string]*Archive)
					}
					archive.SubArchives[header.Name] = subArchive
				}
			case matchers.TypeRar:
				subArchive, err := AnalyzeRar(newReader)
				if err == nil {
					if archive.SubArchives == nil {
						archive.SubArchives = make(map[string]*Archive)
					}
					archive.SubArchives[header.Name] = subArchive
				}
			case matchers.TypeZip:
				content, err := ioutil.ReadAll(newReader)
				if err == nil {
					subArchive, err := AnalyzeZip(bytes.NewReader(content), uint64(len(content)))
					if err == nil {
						if archive.SubArchives == nil {
							archive.SubArchives = make(map[string]*Archive)
						}
						archive.SubArchives[header.Name] = subArchive
					}
				}
			}
		}
	}
}
