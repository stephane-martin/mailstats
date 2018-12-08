package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"github.com/inconshreveable/log15"
	"github.com/nwaples/rardecode"
	"github.com/xi2/xz"
)

func AnalyzeArchive(typ types.Type, reader *bytes.Reader, size uint64, logger log15.Logger) (*models.Archive, error) {
	switch typ {
	case matchers.TypeZip:
		return AnalyzeZip(reader, size, logger)
	case matchers.TypeTar:
		return AnalyzeTar(reader, logger)
	case matchers.TypeRar:
		return AnalyzeRar(reader, logger)
	default:
		return nil, errors.New("unknown archive type")
	}
}

func replaceCompressed(oldType types.Type, oldReader io.Reader, logger log15.Logger) (types.Type, io.Reader, string, error) {

	switch oldType {
	case matchers.TypeGz:
		r, err := gzip.NewReader(oldReader)
		if err != nil {
			return oldType, nil, "", err
		}
		newt, newr, err := utils.GuessReader(r)
		if err != nil {
			logger.Info("Failed to determine inner type of compressed file in archive", "error", err)
			return oldType, newr, "gzip", nil
		}
		return newt, newr, "gzip", nil
	case matchers.TypeBz2:
		r := bzip2.NewReader(oldReader)
		newt, newr, err := utils.GuessReader(r)
		if err != nil {
			logger.Info("Failed to determine inner type of compressed file in archive", "error", err)
			return oldType, newr, "bzip2", nil
		}
		return newt, newr, "bzip2", nil
	case matchers.TypeXz:
		r, err := xz.NewReader(oldReader, 0)
		if err != nil {
			return oldType, nil, "", err
		}
		newt, newr, err := utils.GuessReader(r)
		if err != nil {
			logger.Info("Failed to determine inner type of compressed file in archive", "error", err)
			return oldType, newr, "xz", nil
		}
		return newt, newr, "xz", nil
	default:
		return oldType, oldReader, "", nil
	}
}

func AnalyzeZip(reader io.ReaderAt, size uint64, logger log15.Logger) (*models.Archive, error) {
	zipReader, err := zip.NewReader(reader, int64(size))
	if err != nil {
		return nil, err
	}
	archive := new(models.Archive)
	archive.ArchiveType = "zip"

LoopFiles:
	for _, f := range zipReader.File {
		entry := models.ArchiveFile{Name: f.Name, Extension: filepath.Ext(f.Name)}
		archive.Files = append(archive.Files, &entry)
		archive.DecompressedSize += f.UncompressedSize64
		if f.FileInfo().IsDir() {
			continue LoopFiles
		}

		fileReader, err := f.Open()
		if err != nil {
			logger.Warn("Error reading file from ZIP", "error", err)
			continue LoopFiles
		}
		t, newReader, err := utils.GuessReader(fileReader)
		if err != nil {
			logger.Info("Failed to detect file type from ZIP archive", "error", err)
			_ = fileReader.Close()
			continue LoopFiles
		}

		t, newReader, entry.Compression, err = replaceCompressed(t, newReader, logger)
		if err != nil {
			logger.Info("Failed to decompress file from ZIP archive", "error", err)
			_ = fileReader.Close()
			continue LoopFiles
		}
		entry.Type = t.MIME.Value
		switch t {
		case matchers.TypeTar:
			subArchive, err := AnalyzeTar(newReader, logger)
			if err == nil {
				if archive.SubArchives == nil {
					archive.SubArchives = make(map[string]*models.Archive)
				}
				archive.SubArchives[f.Name] = subArchive
			}
		case matchers.TypeRar:
			subArchive, err := AnalyzeRar(newReader, logger)
			if err == nil {
				if archive.SubArchives == nil {
					archive.SubArchives = make(map[string]*models.Archive)
				}
				archive.SubArchives[f.Name] = subArchive
			}
		case matchers.TypeZip:
			content, err := ioutil.ReadAll(newReader)
			if err == nil {
				subArchive, err := AnalyzeZip(bytes.NewReader(content), uint64(len(content)), logger)
				if err == nil {
					if archive.SubArchives == nil {
						archive.SubArchives = make(map[string]*models.Archive)
					}
					archive.SubArchives[f.Name] = subArchive
				}
			}
		}
		_ = fileReader.Close()
	}
	return archive, nil

}

func AnalyzeRar(reader io.Reader, logger log15.Logger) (*models.Archive, error) {
	rarReader, err := rardecode.NewReader(reader, "")
	if err != nil {
		return nil, err
	}
	archive := new(models.Archive)
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
		entry := models.ArchiveFile{Name: header.Name, Extension: filepath.Ext(header.Name)}
		archive.Files = append(archive.Files, &entry)
		if !header.UnKnownSize {
			archive.DecompressedSize += uint64(header.UnPackedSize)
		}
		if header.IsDir {
			continue LoopFiles
		}

		t, newReader, err := utils.GuessReader(rarReader)
		if err != nil {
			logger.Info("Failed to detect file type from RAR archive", "error", err)
			continue LoopFiles
		}
		t, newReader, entry.Compression, err = replaceCompressed(t, newReader, logger)
		if err != nil {
			logger.Info("Failed to decompress file from RAR archive", "error", err)
			continue LoopFiles
		}
		entry.Type = t.MIME.Value
		switch t {
		case matchers.TypeTar:
			subArchive, err := AnalyzeTar(newReader, logger)
			if err == nil {
				if archive.SubArchives == nil {
					archive.SubArchives = make(map[string]*models.Archive)
				}
				archive.SubArchives[header.Name] = subArchive
			}
		case matchers.TypeRar:
			subArchive, err := AnalyzeRar(newReader, logger)
			if err == nil {
				if archive.SubArchives == nil {
					archive.SubArchives = make(map[string]*models.Archive)
				}
				archive.SubArchives[header.Name] = subArchive
			}
		case matchers.TypeZip:
			content, err := ioutil.ReadAll(newReader)
			if err == nil {
				subArchive, err := AnalyzeZip(bytes.NewReader(content), uint64(len(content)), logger)
				if err == nil {
					if archive.SubArchives == nil {
						archive.SubArchives = make(map[string]*models.Archive)
					}
					archive.SubArchives[header.Name] = subArchive
				}
			}
		}
	}
}

func AnalyzeTar(reader io.Reader, logger log15.Logger) (*models.Archive, error) {
	tarReader := tar.NewReader(reader)
	archive := new(models.Archive)
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
		entry := models.ArchiveFile{Name: header.Name, Extension: filepath.Ext(header.Name)}
		archive.Files = append(archive.Files, &entry)
		archive.DecompressedSize += uint64(header.Size)
		if header.Typeflag != tar.TypeReg {
			continue LoopFiles
		}
		t, newReader, err := utils.GuessReader(tarReader)
		if err != nil {
			logger.Info("Failed to detect file type from TAR archive", "error", err)
			continue LoopFiles
		}
		t, newReader, entry.Compression, err = replaceCompressed(t, newReader, logger)
		if err != nil {
			logger.Info("Failed to decompress file from TAR archive", "error", err)
			continue LoopFiles
		}
		entry.Type = t.MIME.Value
		switch t {
		case matchers.TypeTar:
			subArchive, err := AnalyzeTar(newReader, logger)
			if err == nil {
				if archive.SubArchives == nil {
					archive.SubArchives = make(map[string]*models.Archive)
				}
				archive.SubArchives[header.Name] = subArchive
			}
		case matchers.TypeRar:
			subArchive, err := AnalyzeRar(newReader, logger)
			if err == nil {
				if archive.SubArchives == nil {
					archive.SubArchives = make(map[string]*models.Archive)
				}
				archive.SubArchives[header.Name] = subArchive
			}
		case matchers.TypeZip:
			content, err := ioutil.ReadAll(newReader)
			if err == nil {
				subArchive, err := AnalyzeZip(bytes.NewReader(content), uint64(len(content)), logger)
				if err == nil {
					if archive.SubArchives == nil {
						archive.SubArchives = make(map[string]*models.Archive)
					}
					archive.SubArchives[header.Name] = subArchive
				}
			}
		}
	}
}
