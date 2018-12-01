package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"github.com/nwaples/rardecode"
	"io"
	"io/ioutil"
)

type Archive struct {
	Filenames          []string            `json:"filenames,omitempty"`
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

func AnalyzeZip(reader io.ReaderAt, size uint64) (*Archive, error) {
	zipReader, err := zip.NewReader(reader, int64(size))
	if err != nil {
		return nil, err
	}
	archive := new(Archive)
	archive.ArchiveType = "zip"
	for _, f := range zipReader.File {
		archive.Filenames = append(archive.Filenames, f.Name)
		archive.DecompressedSize += f.UncompressedSize64
		fileReader, err := f.Open()
		if err == nil {
			t, newReader, err := Guess(fileReader)
			if err == nil {
				if t == matchers.TypeGz {

				}
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
	for {
		header, err := rarReader.Next()
		if err == io.EOF {
			return archive, nil
		}
		if err != nil {
			return archive, err
		}
		archive.Filenames = append(archive.Filenames, header.Name)
		if !header.UnKnownSize {
			archive.DecompressedSize += uint64(header.UnPackedSize)
		}
		t, newReader, err := Guess(rarReader)
		if err == nil {
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
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return archive, nil
		}
		if err != nil {
			return archive, err
		}
		archive.Filenames = append(archive.Filenames, header.Name)
		archive.DecompressedSize += uint64(header.Size)
		t, newReader, err := Guess(tarReader)
		if err == nil {
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
