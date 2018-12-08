// Code generated by go-bindata. DO NOT EDIT.
// sources:
// data/stopwords-en.txt (3.628kB)
// data/stopwords-fr.txt (4.05kB)

package extractors

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes  []byte
	info   os.FileInfo
	digest [sha256.Size]byte
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _dataStopwordsEnTxt = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x4c\x57\x5d\xba\xe5\xa0\xaa\x7c\x67\x22\xfd\x74\x07\x65\x22\x89\xde\x65\x24\x47\x70\xa5\x3d\xa3\x3f\x5f\x41\x76\x77\x3f\xec\xaa\x8a\x0b\xff\x10\xc1\x9d\xb6\xc6\x94\x36\x99\x06\xfc\x42\xab\x51\xda\x77\x19\x39\xf5\x9d\x5f\x59\xfb\xf9\x57\xb5\x45\x69\x1f\xa2\x4a\x69\xb7\x99\x1a\xbe\x73\xe6\x4c\xe9\x38\x78\xb7\x3f\xc2\x3b\xb9\x52\x4a\x87\xf1\x08\x7c\xd2\xc8\x4a\xe9\x4c\xb5\x07\x62\xc2\x76\x89\x93\x74\x76\x3c\x29\xb5\xc1\x29\x2f\x4a\x4d\x85\x52\xb3\x22\xf3\x2c\x94\xda\x93\x96\x52\xba\xdc\x04\x88\x6e\xbd\xcb\xf4\xc5\x76\xb1\x82\x79\xfa\xda\x04\x7d\xfb\x2a\xf2\x80\x2e\x19\xf8\x79\xf9\xf8\x7d\x59\xf1\xc5\xf5\xf5\xa4\xf5\x92\x3a\x17\x86\xdd\x7d\xa7\xc1\xdd\xb0\xb1\xfb\x1e\xf2\xbb\x5e\xc9\x18\x5f\x83\xbb\x83\x51\x1a\x55\x99\xd2\x90\xd9\x33\x25\xad\x99\x29\xe9\xc7\x47\x9d\x56\x28\x7d\x53\x6d\xe1\x5c\x9f\xe2\x39\x26\xfc\xb4\xa5\xfd\x43\x1b\xef\xe9\x62\xa7\xa9\xce\x72\xfd\x90\x06\x63\x9c\x8d\xb9\xd3\xc6\x07\x56\x1e\x54\x52\xcf\xb4\xf1\x59\x7b\x60\x0f\xb3\x57\x69\x48\x50\xa9\x6e\x18\x3f\xb7\xca\x5f\x8c\xd0\xe4\xa1\x8d\x7d\xa5\x41\xb0\xb4\x27\x66\x59\x82\x1e\x55\x1a\x6d\x62\x85\xb6\x51\xf9\x08\x6c\x8b\x7c\xb9\x3b\xbc\x6c\xa0\x5f\x40\xac\xdc\x51\x69\xe7\x61\x38\xcc\x97\xd1\x01\x1b\x8a\xed\xec\xd2\xe3\xc7\x60\x2c\xe9\x95\xf8\x6d\xb6\x1c\xd8\x8d\x72\x32\xa6\x5c\x33\xc6\xcf\xf5\x38\xd8\xfd\x9c\x85\xd5\xc1\x9b\x05\xfd\x33\x0e\x31\x4b\x34\x3c\xdd\x21\x82\x2a\xcf\x01\x03\x4e\x7b\x21\xf6\xc8\x23\xae\x67\x79\x71\x11\x57\x0f\x0f\x6e\xca\x0e\x71\xdc\xdc\x3d\xbe\xb9\x7b\x84\xb1\xde\xbc\x57\x8f\x6a\xb6\xff\x4b\x8d\xf8\xcb\x1d\x30\x1c\x56\xa0\x87\x97\x2b\x2c\xc6\x45\xc4\x94\xcb\x77\xdc\xdf\x3b\xdf\x46\x47\x3d\xac\xd0\x51\x87\x42\x7f\x99\x0e\x69\x4d\x1e\xce\xaf\x40\xaf\x50\x4a\x87\x8c\x8b\xc7\x4b\x6d\x41\xa0\xaf\x07\xd9\x21\x73\xd0\x31\xe4\xa2\x63\x0e\xdf\xc7\xcb\x1e\xdb\x67\xfa\x32\x9d\x6c\x0a\xf0\xab\x77\x62\x2e\x40\x77\x54\xa0\xb7\xc3\xa3\x27\x96\x7d\x8a\x19\x77\x2a\xe9\xbe\xb9\x2b\x95\x34\x72\x5b\x54\x92\xfb\xba\x60\x40\xc0\xfb\x81\xae\x85\x71\xcb\x7c\x73\x80\xb8\xd4\x50\xdb\x72\xaa\xdd\x49\x1d\xe7\x2d\xfe\xe9\x5f\xca\xed\xa0\x52\xaf\x97\x7d\xf9\x05\x51\x52\xe4\xd9\xb8\x1a\xd8\x7d\x5c\x66\xcf\x83\x33\xd5\x5f\xad\x51\xbd\x2e\xce\x15\x71\xf1\x47\xb5\x45\xf5\xba\x65\x98\x67\xa7\x1f\x69\x54\x7b\x66\x74\xeb\x99\x7f\x53\xed\xf0\x60\xb2\x2a\x9d\x90\x5f\x38\xe1\x17\x13\xaa\xfd\xcb\xfd\x6d\x46\xc8\x50\xf5\xbd\x56\xf3\xd9\xcc\x57\x57\x7f\x7d\x99\xfe\x7f\xaa\xd1\x87\xf9\x76\x50\xfa\xe0\x24\x3f\x5d\x1e\x87\xee\xa8\xd4\xd2\x38\xb1\xa2\x96\xd4\xa8\xc5\xea\x40\x03\xf8\x97\xd0\xca\x6e\xc2\xaa\x00\x28\x53\x6a\xf5\xc3\x0e\xd9\x11\x66\xb5\xa3\xc5\xac\x31\x35\x91\x8f\x03\x3c\x0f\x56\xba\x52\x66\xba\x52\x6d\x26\x20\x5c\xb5\x2b\x7d\xd8\x01\xbf\x76\x7c\xaf\x8d\xe9\xe2\xd4\x1d\xd4\xd1\xea\x15\x6d\x4f\xa9\x0d\x6a\x60\xb2\xcb\xaf\xc6\x55\x5b\x83\x43\xae\xaa\x4a\x1e\x4a\x00\xc1\x59\x78\x4e\x06\xc0\x78\xee\x85\x2e\x78\xe5\x5a\xee\xa6\x8e\xa4\x00\x68\x8b\x3a\xa7\xe1\xe0\x7a\x67\xd5\x34\xea\x3f\x1a\x8a\xb3\x83\x52\x7f\x6f\x61\xf7\xf3\x76\xb4\xc2\xee\x9a\xce\xbf\x8d\x3a\x7c\x00\xb0\x45\x5d\xfc\xa2\x75\x44\x2b\xe0\x8f\xa1\x44\xcb\xb8\xfc\xa2\x76\x41\xd5\x41\xf2\x87\xaf\xba\xc4\xfd\x93\xcd\x33\x4f\x10\x67\x92\xed\x5b\x65\x6a\x5b\x24\x07\xe2\x5e\x3e\x69\x91\x5c\xd5\xd0\x59\x10\x4e\xd2\x59\x49\xe0\x56\x41\xb4\x44\x31\x71\xd4\xa0\x07\x49\x1f\x69\xc2\x48\x26\x1a\x27\x22\x1b\x97\x4b\xa6\x79\x6a\x75\xc7\x01\x52\x6b\x14\x97\xfb\x4e\x27\x3b\x28\xdd\x69\x98\x43\xdd\x67\x4b\xe3\x1f\xd9\x16\xdd\x08\x91\x9b\x47\x49\xb7\xd2\xdd\xd2\xce\x99\x6e\x04\x0e\xd3\xdd\xa6\xd2\x2d\xe2\x66\xa2\x5a\x51\x59\x5e\x81\x16\x43\x54\xbb\x2b\xee\xc1\x19\xf5\x23\x79\xf9\xba\x07\x2b\xb2\xe8\x3d\xf8\x67\xef\xf7\xa8\x57\x9c\xce\x3d\x64\x4b\x5b\x88\xeb\x0e\x73\x99\x19\xf8\xf5\xf2\xf0\x9f\x59\xf7\x4f\x5b\x60\x63\x1a\xc9\xdd\x81\xaa\x8c\xde\x83\x7d\xbe\xc1\x3b\x26\x08\xf2\xef\x43\x69\xf0\x99\xe2\xd1\x10\xca\xcf\x2c\x24\x18\x57\x24\x07\xd7\x2f\x7b\x27\xe5\x34\xf6\x02\x71\xe3\xe5\xf0\xd3\x3a\x5b\x58\x42\xc4\x70\x50\x4a\xc3\x43\x77\xcc\x4e\x9a\x6a\x26\x45\x2c\x6a\x5a\x30\x51\x94\x72\xc5\x20\xd2\x49\xd9\x8b\xa0\x32\x5f\x0e\x9c\x9d\x7e\xda\x60\xc8\xb0\x6a\x07\xbd\xe7\xe8\xee\x52\xcf\xf9\xc0\x91\x1a\x69\xc1\x59\x6a\x41\xe7\xc2\xbf\x42\x2b\x69\xf1\xf2\x15\x84\x2c\xa2\x78\x6b\x00\xdc\x10\x59\xc2\xd1\x2d\x1f\x25\xad\x67\xaf\x47\xdd\x91\xad\xfe\xd1\x6d\x91\xd6\xab\x22\x1a\x5e\xf6\x16\x84\xa3\x36\x6c\x13\x9f\xc8\x94\x00\xbf\x0d\x10\x3e\x95\x5c\x8c\x6b\x00\xb6\x92\xfa\x2b\x7c\x73\x50\xf5\xed\x04\xa1\xae\x9e\x92\xec\x15\xb8\x20\x2a\x70\x91\x8c\xb1\xc8\x8b\x1e\x16\x84\x33\x7d\x3f\xb0\x0d\x57\x3f\x2d\xe1\x5f\xab\xf0\x80\xc9\x4d\x6a\x43\xfc\x39\xa8\x73\x53\x64\xe1\x08\x41\x9d\x3b\xae\x7d\xbc\x78\x14\x79\x43\xe7\x71\xd4\xbd\x46\x84\xe8\x3c\x4f\xa4\x40\x9d\x83\xc9\x90\xbe\x00\x1d\x88\xf1\x8d\x5b\x23\xe3\x9e\x95\x7c\x53\x80\x4f\x60\xb4\xfc\x06\x9a\x03\xce\x02\xac\xf1\xf5\x65\xb2\xc2\x75\x04\xa2\x91\x2f\x87\xf7\x6c\xad\x60\x9a\x28\x62\xe6\x1e\xb0\xbf\x65\xcc\xde\x3a\xe6\x9c\x83\xfc\xed\x65\x6f\x65\x73\x8e\x19\x79\xb0\x1c\xc1\x3f\x06\x1a\x64\x12\xec\xc5\x2f\x7a\xc4\xaa\xd4\xd1\x87\x5f\x3e\xfa\x7a\x87\x5a\x31\xc2\x0a\xbb\xea\x7b\xad\x18\x4d\xbc\x8b\x4c\x7a\x9f\xbe\x41\xc1\x8a\x97\xa0\x95\x21\xf3\x7c\xa9\xfc\x30\x9e\xf2\x56\x06\xba\x4d\x25\x93\x93\xfd\xda\x9a\x08\xfe\x3e\x64\xe2\x85\x2f\x48\xc9\x06\xce\x19\x08\x3d\xdb\x22\x1b\x7e\xce\xf6\xd4\x9d\x69\xf6\xcc\x83\x26\xea\xa9\xcd\x1e\x15\x6e\x76\xbf\xcf\xb3\x7b\xfd\x0a\xf2\x66\xab\x0d\x28\xe4\xbb\x9f\xca\x19\x70\xcc\xf6\x12\x8c\x5c\x74\xef\x8f\xb7\xe3\x54\xcc\x35\x35\xfe\x8d\xf8\xa6\x36\x99\xbe\x69\x20\x57\x91\xbf\xb6\xbe\xd2\x94\x1e\xdc\x19\x00\xa4\xba\x5e\xf4\x70\xf3\x77\xe6\xe3\x87\xf2\xe0\xde\x3e\x38\x8e\x27\x1e\x8e\x8f\x7b\xde\x23\x1e\xe0\xc5\xe6\x79\x63\xe6\xf1\x98\x79\x10\x0e\x4f\x84\x03\xe8\x35\xf1\x31\xfe\xc6\x45\x48\x0d\xde\x56\x70\xed\xc1\x6f\xb3\xef\xd7\xd5\x3b\x84\xbb\xfc\x29\x75\x2f\x14\x25\xf7\x29\xf5\x02\xbc\x3f\x48\x06\xbc\xd6\xe2\xbf\x4b\x2c\x4c\x2e\x87\x9f\x9f\xd4\x81\xe9\xa9\x19\x3e\x7e\x50\xac\xfb\x49\x4f\xd5\x42\x4f\xb5\x00\xac\xa6\x9a\x9f\xfc\x23\xd8\xba\xe0\x64\x1f\x19\x2d\xd3\xe3\x49\xea\x79\xdf\xd8\x0b\xf9\x7d\xc9\xc4\x5c\x0b\xef\x49\x00\x3b\x6a\x20\x52\xe1\xfa\x5b\xd6\x60\xfb\x65\xfa\x2f\x0f\xa1\xff\x05\x00\x00\xff\xff\xc3\xa8\x6f\xd7\x2c\x0e\x00\x00")

func dataStopwordsEnTxtBytes() ([]byte, error) {
	return bindataRead(
		_dataStopwordsEnTxt,
		"data/stopwords-en.txt",
	)
}

func dataStopwordsEnTxt() (*asset, error) {
	bytes, err := dataStopwordsEnTxtBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "data/stopwords-en.txt", size: 3628, mode: os.FileMode(420), modTime: time.Unix(1544284804, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x41, 0xda, 0x88, 0x1c, 0x42, 0x14, 0xde, 0xf0, 0x26, 0x25, 0x18, 0x1a, 0x9b, 0x45, 0xd6, 0x23, 0x3b, 0x48, 0xe5, 0x7e, 0xbb, 0x91, 0x2d, 0x3e, 0x61, 0x10, 0xd, 0x88, 0x3c, 0x7c, 0xbe, 0x9f}}
	return a, nil
}

var _dataStopwordsFrTxt = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x54\x57\x31\x96\xe4\x2a\xcf\xcd\xb5\x91\x17\xf5\xa2\x28\x2c\x77\x69\x0e\x06\xb7\x04\x3e\xd5\xbd\x9a\x97\xfd\x53\x13\x4f\xf6\x42\x6f\xec\x3f\x57\xc2\xd5\xfd\x25\xf7\x5e\x04\x05\x32\x48\x82\x4a\xb7\xa6\x0b\xa5\x9b\xb5\x32\x36\xae\x9d\xd2\x2a\x95\x92\xb8\x14\x36\x4a\x52\x0a\x0f\x85\xa8\x26\x94\x4a\x99\x9d\xa5\x34\x87\x6a\xa0\xf3\x2f\xa5\xd2\x30\xac\x76\x56\xe1\xa1\xdf\x8a\x7f\x48\xa3\xb4\x4f\x3c\x7f\x1b\x25\x33\xfe\xa2\xd4\x3b\xd7\x65\x50\x1a\x79\xd4\x40\x0e\x32\x4a\xe3\x57\x1b\x70\x31\xf8\x9f\xfb\x10\x4a\x23\xe6\x18\x1f\x83\x0b\xa5\xa1\xc9\x41\x02\xdd\x3b\x08\x0b\x8a\x96\x37\xb0\xd6\x50\x99\xe4\xae\x0f\x7d\x11\x06\x1a\xbe\x71\x74\xf8\x0c\x5c\x9b\xcf\xd2\x95\x63\x73\xa0\xc2\xe0\x6e\x3c\xe0\x40\x71\x8b\x4b\xa3\x74\x4c\x07\x0e\x5f\xff\xf0\xe5\x8f\xe4\x16\xce\x80\x2f\x4a\x87\x04\xfa\xca\x47\x13\x05\x42\x7f\xfa\xc0\x4f\xf4\x7e\xc2\x70\x4b\xc6\x4c\x37\x4e\xc3\x21\xb7\xb1\xd3\x4d\xb8\xd2\x4d\xde\x95\xe9\xd6\xc6\x46\x37\x4d\x47\xa3\x9b\xaa\x52\xe6\x2c\x94\x79\xe1\x07\x65\x2e\x09\x50\x38\xf0\xcd\x7b\x20\xca\xf9\x6f\x28\x9b\xf4\xea\xb2\xab\x6f\x48\xe0\xec\x19\x72\x75\x38\xd6\x4e\x99\x77\xae\x4b\x72\xa5\x3d\x49\xbd\x98\x5f\xc2\x2e\x15\xc2\xdb\xbd\xa3\x7f\x3c\x1c\x62\xf2\xf1\x88\xb9\xef\x09\x47\x1f\xc4\xe0\x8f\x01\x62\x75\x30\xe0\x17\xe5\xbb\xe4\x3b\xcc\xa3\x53\xbe\x9f\xbf\x95\x27\x19\x65\xa9\x1f\x0e\x23\xd5\xe9\xc8\x6c\x7c\x2b\x39\x7f\x6f\xb3\x35\x65\x49\x99\x72\x91\x4c\xb9\x6d\xbe\xaf\xb9\x6d\xb0\x03\xf1\x71\x6d\xdb\x93\xa6\x5b\xe1\x1f\xd2\x5c\xab\x80\x6b\x66\xad\xbe\x0d\xad\x22\x66\x72\x1b\x98\x4c\x53\xa6\x25\x55\xa3\x85\x6f\x6d\x74\x5a\x78\xb6\xee\xc8\x90\x85\x7f\x25\x5a\x7c\x33\x17\xde\x87\xc0\xa2\x55\x58\x2f\x66\x08\xfd\x16\xfe\xa1\x0b\x5b\xd3\x2d\xf9\x68\xbb\xc2\x6e\x4a\x17\xd6\x46\xb0\xd3\x78\x38\xc4\x77\xbe\x94\x7f\xd5\xc2\x1e\x8f\x0b\x1f\xec\xde\x1c\x9a\x02\xa5\xd3\x22\xeb\xca\xea\xa3\x2e\x85\x65\x2e\x1d\xf2\x7c\xbe\x46\x84\xe4\x9f\xfa\xe7\x18\x68\xf8\x2e\xca\xb9\xbf\x38\xbc\x10\x37\xf8\x90\x70\xc4\x89\x2f\x86\xe1\xf1\x76\x1f\xee\xd4\xe3\xad\xf2\x58\x5d\x18\xef\x6e\x99\x5f\xd6\xd0\xdf\xe4\xf0\x29\x5b\xcd\x00\xa8\xf1\xc5\x8e\x73\x98\x4a\x7d\xa7\x45\x9b\x2f\x1a\x65\x63\x19\xea\xdb\x70\x3e\x6f\xc3\xe9\xda\x5f\x5e\x57\xee\xc4\xef\xa9\x70\xa0\xfb\xeb\xca\xc8\x33\xca\xf3\x68\x3b\xff\x6c\x21\xc3\x6a\x61\x31\xe2\x9a\x9b\x32\x71\x45\x25\x65\x8f\x0b\xae\xfe\x89\x5c\x0f\xd1\x56\x89\xcd\x92\x10\x77\xac\xcf\xde\x3f\xf0\xbb\xe1\xdb\xca\xc3\x8c\x03\xaf\x96\x39\xa1\x68\x38\xa3\x32\x20\x71\xae\xf5\x1e\xe9\xda\x54\x7e\x64\xde\xfb\xf9\x24\x7e\x74\xae\xd6\x9c\xa3\x1a\xf3\xf9\x9f\x0f\x3e\xff\xc3\x11\xad\xf8\x50\x40\x54\x2b\x57\xc1\x01\x3e\xe4\xfc\xbf\x56\x69\x65\xaf\x8c\x2b\x52\x65\x2d\x2d\x93\x97\xc4\xd5\x6d\x4d\x33\xd3\x1a\x6e\xaf\xee\xf6\x3a\xdd\x5e\xc3\xed\x75\xba\xbd\x5e\x6e\xaf\xe1\xc5\x1a\x5e\xbc\x73\x35\xba\xa7\xd1\xe9\xce\x52\xe9\xde\x90\x13\xf7\xa6\x9b\x18\x79\xaa\xdc\x51\xf0\x3c\x06\x00\x71\x96\xf7\xa1\x9a\xee\x74\x3f\x9f\x25\x19\xc9\xb6\x37\xed\x4c\xbf\x86\x7d\x8c\x40\x6f\x74\xa6\x92\xc4\x8c\x95\x4a\x8a\x74\xa1\xc2\x7e\xf0\xe5\x3b\x7f\xca\x95\x3f\xb8\xe2\x28\xee\xb9\xd2\xea\x7b\xe7\x6d\x87\xd2\x00\xcc\x89\x2a\x88\x04\x0a\xe1\x67\xbf\x25\x29\xbd\x81\x6a\x0f\x64\x2f\x06\x1e\x45\x5b\x2a\x28\xd1\x4e\xe7\x93\xb6\xf4\x90\x0d\x11\xe5\x73\x00\x8c\x36\xd6\x2c\xb4\xa1\xee\x00\x2a\x4f\x32\x67\x20\xbc\xde\xa4\x66\x47\x87\x39\x49\x9b\xce\xb8\x08\x67\x9a\xd4\x05\x95\x10\x02\xf8\xc9\x35\xbc\x19\xa5\xcb\x8e\x1f\x4d\x61\x34\x7f\x11\xe1\x53\x53\x1f\xca\xe5\xe2\xc2\xdf\xca\xa8\x72\xaa\x31\x61\xe5\xec\x61\xab\xfc\x43\x7a\xd0\x79\x6a\x56\x1e\x47\x9c\x4f\x6d\xdb\x4d\x79\xe0\xf8\xa7\x7c\x40\x6c\xe7\x13\x86\x9e\xa2\xb4\xd6\x86\xb8\xaf\xa8\x59\x80\x2b\x96\x6b\x1b\x07\x6e\xbb\xc9\x0f\xaa\xe7\xf3\xe5\xc1\xf9\xd7\x7f\xe3\x64\xd4\x4a\x39\x9f\xd4\xea\x17\x03\x62\xed\xe6\x1d\x43\x12\xd0\xba\x03\x53\xf3\xfb\x1c\x33\x6a\x9f\x74\x35\x8d\xf6\x84\x20\xde\x93\x7a\x5c\xef\x49\x0b\x07\xd6\x1e\xac\xa0\x4d\x80\x2d\xfa\x0c\x3b\xbf\x27\xf5\xfc\x05\x4b\x1e\x45\x62\x60\x68\xaf\xd8\xff\xd3\xda\x62\x3e\xb3\xf3\x49\xd7\xdd\xb9\x73\x35\xa6\x9d\x75\x63\x34\xd4\x1a\x62\xe0\x12\x46\x3b\x0f\xd8\x87\x17\xb7\x1d\xfb\xb8\xaf\x6b\xa7\x7d\x85\x19\x07\xb1\xcb\xf9\x1b\xce\x17\x64\xcf\x5e\xda\x58\x69\x2f\x03\xeb\x82\xcd\x41\x3c\xa8\xf7\x32\xfa\xf9\xb7\xd3\xde\x90\x97\x26\xeb\xb7\x32\x97\x82\x4b\xee\x12\x30\x8d\x74\x07\xaa\xc3\xc7\x68\xe2\xc2\x5f\x55\x53\x60\xb6\xe1\x0f\x9c\x5d\x39\x15\xbf\x26\x77\xe5\x2c\x36\xbf\x56\x79\xf3\x5d\x01\xc7\x8e\x5c\xea\xea\x37\xda\xb5\xdd\xe6\x2f\xdb\xcd\x6f\xeb\x5d\x5b\xe6\xd8\x1f\x6d\xb8\xee\xe3\xa5\xb8\x9b\xf4\x4e\x7e\x5d\x02\x90\x92\x3b\x9e\x96\xb8\xd9\x17\xc7\x1e\xf8\x76\xfe\xfb\x66\x4d\xa2\x91\x40\xea\xf3\x7e\x8c\xd4\x9b\x7e\x85\xd0\x8b\xde\x0e\xa9\xef\x3d\x1a\x11\x43\xdf\xd2\xbd\xf4\x92\x01\xc8\xad\x62\xd1\x59\x4b\xae\x0a\x02\xfe\x18\xff\x8c\x3a\x15\x5f\x1c\x5d\x40\x79\xfd\x52\xaa\x2f\xef\xce\x35\x81\x4d\x93\x06\xf8\x5a\x10\x46\xca\x25\x75\x39\xf8\x25\xa2\x8f\xb7\xa4\x1f\xc3\xf7\x4a\xb9\x2e\x0e\xf8\x31\x9b\xc7\x21\x38\x5a\x3c\x5b\x2a\xb9\xcb\x4a\xca\x1d\xc7\xa8\x7c\x34\xc9\x12\x8c\x3a\xab\xa8\x3b\x96\xb2\xf2\xad\xf0\x20\xc3\x41\x1a\x9e\x29\x96\xf0\xba\xe9\x42\x96\xc6\x4a\x86\xd8\x32\x96\x2f\x26\xe3\xd2\xa0\xb7\x5b\x1c\x76\x28\xbf\x40\x5c\x4e\x4b\xb4\x1d\xf6\x80\xd8\x58\x63\x4d\x0e\x12\x18\x43\x3c\xa2\x9c\xa2\xe5\x0d\xfe\x02\xca\x24\x5c\x1c\xc6\xfa\x22\x0c\x1c\xc5\x81\x03\x7d\x87\xcc\x3f\x28\x0a\xa9\xcd\x42\x6a\x5e\x48\x4d\x2a\x1c\xbf\x1e\x0d\xf6\x2a\x9a\xd6\xc2\x0b\xa4\xbe\xe1\x29\x61\x4d\x1e\x1e\x2d\x86\x27\x20\x8c\xde\x3d\xa0\x22\x0f\xad\xe1\x79\x6e\xcd\xdf\xe7\xb6\x73\x96\xd5\x4f\xf2\x5b\x86\x79\xe0\xec\x56\xb2\xde\x76\x8a\xb3\x98\x5e\x8e\x5b\x17\x44\x8e\x8d\x75\x8d\x5b\xf7\xa5\x38\x24\x2c\x70\x68\x84\x3a\x62\x8c\x33\xbf\x84\x5d\x2a\x84\xa2\xe7\x17\x63\xe0\xce\xba\x37\x43\x5b\x3b\x5e\x9f\x3d\xd5\x45\x8c\x3c\x4a\x7a\x52\xbc\xaf\xa8\x7b\x10\x3b\xba\x57\x3d\xa2\xb9\x23\x62\xfb\xf9\x2c\x34\xaf\x32\xfc\x2f\x03\x88\x02\x3b\x53\xc7\x26\xf7\xd8\xe4\x3e\x37\xb9\xfb\x26\xf7\xd7\xae\xf6\x36\xf2\xdd\x7f\xdd\xfc\x4f\x1b\xfa\x86\x83\x9b\x3a\x07\x7a\xc1\x75\x65\xd4\xd5\x23\xac\xc7\x6b\xd2\xeb\x7b\x57\xef\x07\xc6\xb1\x7d\xcb\x70\x59\xdb\x4e\xdd\x6b\x43\xb7\x26\x15\x38\xa4\xd2\x80\x4b\xa3\xca\xda\x74\x8e\x1c\xd5\x4f\x28\xc8\xc8\xff\x99\x1d\xc9\xaf\x7c\x7f\x96\x1d\x28\x82\x51\x07\x0e\x39\x92\x23\x3b\x18\x1d\x25\x55\x8a\xcc\x39\x9a\xb0\x43\xed\x34\x73\xe8\x80\x8b\x07\x02\xe7\x68\x6e\x45\x61\x39\xf0\xb1\xc7\x8f\x4b\xed\x88\x8b\xeb\x98\x17\xd7\x81\x47\xe8\x67\xaa\x95\xce\x67\x8f\x34\x70\x61\x41\xd1\x9a\x46\x47\xe4\x01\x08\x11\x77\x3e\xfb\xf9\xe4\x49\xb3\x69\x74\xfe\xc1\x26\x9e\x7f\xb0\xcc\xff\x07\x00\x00\xff\xff\xf8\x9e\xd7\xfc\xd2\x0f\x00\x00")

func dataStopwordsFrTxtBytes() ([]byte, error) {
	return bindataRead(
		_dataStopwordsFrTxt,
		"data/stopwords-fr.txt",
	)
}

func dataStopwordsFrTxt() (*asset, error) {
	bytes, err := dataStopwordsFrTxtBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "data/stopwords-fr.txt", size: 4050, mode: os.FileMode(420), modTime: time.Unix(1544284796, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xc8, 0x3d, 0xcd, 0xfc, 0xee, 0x38, 0x92, 0xb5, 0x1a, 0x64, 0x80, 0xa0, 0x4d, 0x39, 0xf7, 0xd, 0x8, 0x1e, 0x41, 0x83, 0x5b, 0x98, 0xd4, 0xe7, 0x87, 0x39, 0x48, 0x7, 0xe8, 0x85, 0xc2, 0x6c}}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// AssetString returns the asset contents as a string (instead of a []byte).
func AssetString(name string) (string, error) {
	data, err := Asset(name)
	return string(data), err
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// MustAssetString is like AssetString but panics when Asset would return an
// error. It simplifies safe initialization of global variables.
func MustAssetString(name string) string {
	return string(MustAsset(name))
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetDigest returns the digest of the file with the given name. It returns an
// error if the asset could not be found or the digest could not be loaded.
func AssetDigest(name string) ([sha256.Size]byte, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return [sha256.Size]byte{}, fmt.Errorf("AssetDigest %s can't read by error: %v", name, err)
		}
		return a.digest, nil
	}
	return [sha256.Size]byte{}, fmt.Errorf("AssetDigest %s not found", name)
}

// Digests returns a map of all known files and their checksums.
func Digests() (map[string][sha256.Size]byte, error) {
	mp := make(map[string][sha256.Size]byte, len(_bindata))
	for name := range _bindata {
		a, err := _bindata[name]()
		if err != nil {
			return nil, err
		}
		mp[name] = a.digest
	}
	return mp, nil
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"data/stopwords-en.txt": dataStopwordsEnTxt,

	"data/stopwords-fr.txt": dataStopwordsFrTxt,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"},
// AssetDir("data/img") would return []string{"a.png", "b.png"},
// AssetDir("foo.txt") and AssetDir("notexist") would return an error, and
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		canonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(canonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"data": &bintree{nil, map[string]*bintree{
		"stopwords-en.txt": &bintree{dataStopwordsEnTxt, map[string]*bintree{}},
		"stopwords-fr.txt": &bintree{dataStopwordsFrTxt, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory.
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	return os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
}

// RestoreAssets restores an asset under the given directory recursively.
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(canonicalName, "/")...)...)
}
