package utils

import (
	"archive/tar"
	"compress/gzip"
	"github.com/inconshreveable/log15"
	"github.com/oschwald/geoip2-golang"
	"github.com/pkg/errors"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"go.uber.org/fx"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var geoipReader *geoip2.Reader
var geoipFile = "/var/lib/mailstats/GeoLite2-City/GeoLite2-City.mmdb"
var GeoIPURL = "https://geolite.maxmind.com/download/geoip/database/GeoLite2-City.tar.gz"

type GeoIP interface {
	Service
	GeoIP(ip net.IP) (*models.GeoIPResult, error)
}

type geoIPImpl struct {
	reader *geoip2.Reader
}

func NewGeoIP(databasePath string) (GeoIP, error) {
	r, err := geoip2.Open(databasePath)
	if err != nil {
		return nil, err
	}
	return &geoIPImpl{reader: r}, nil
}

var GeoIPService = fx.Provide(func(lc fx.Lifecycle, args *arguments.Args, logger log15.Logger) (GeoIP, error) {
	if !args.GeoIP.Enabled {
		return nil, nil
	}
	s, err := NewGeoIP(args.GeoIP.DatabasePath)
	if err != nil {
		return nil, err
	}
	if lc != nil && s != nil {
		Append(lc, s, logger)
	}
	return s, nil
})

func (g *geoIPImpl) Name() string { return "GeoIP"}

func (g *geoIPImpl) Close() error {
	if g.reader != nil {
		return g.reader.Close()
	}
	return nil
}


func (g *geoIPImpl) GeoIP(ip net.IP) (*models.GeoIPResult, error) {
	if g == nil {
		return nil, errors.New("GeoIP database not loaded")
	}
	c, err := geoipReader.City(ip)
	if err != nil {
		return nil, err
	}
	return &models.GeoIPResult{
		Country:   c.Country.Names["en"],
		Continent: c.Continent.Names["en"],
		City:      c.City.Names["en"],
		Coordinates: &models.LatLon{
			Latitude:  c.Location.Latitude,
			Longitude: c.Location.Longitude,
		},
	}, nil
}


func DownloadGeoIP(dest, u string) error {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		dest = "/var/lib/mailstats"
	}
	u = strings.TrimSpace(u)
	if u == "" {
		u = GeoIPURL
	}
	pu, err := url.Parse(u)
	if err != nil {
		return err
	}
	filename := filepath.Join(dest, filepath.Base(pu.Path))
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	resp, err := http.Get(u)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(filename)
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(filename)
		return err
	}
	err = f.Close()
	if err != nil {
		_ = os.Remove(filename)
		return err
	}
	f, err = os.Open(filename)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(filename)
		return err
	}
	gzipr, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(filename)
		return err
	}
	//noinspection GoUnhandledErrorResult
	defer gzipr.Close()
	tarr := tar.NewReader(gzipr)
	for {
		h, err := tarr.Next()
		if err == io.EOF {
			break
		}
		if h.FileInfo().IsDir() {
			err := os.MkdirAll(filepath.Join(dest, h.Name), 0755)
			if err != nil {
				return err
			}
		} else {
			fname := filepath.Join(dest, h.Name)
			dname := filepath.Dir(fname)
			err := os.MkdirAll(dname, 0755)
			if err != nil {
				return err
			}
			f2, err := os.OpenFile(filepath.Join(dest, h.Name), os.O_CREATE | os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_, err = io.Copy(f2, tarr)
			if err != nil {
				_ = f2.Close()
				return err
			}
			_ = f2.Close()
		}
	}
	ddest := filepath.Join(dest, "GeoLite2-City")
	err = os.RemoveAll(ddest)
	if err != nil {
		return err
	}
	f3, err := os.Open(dest)
	if err != nil {
		return err
	}
	//noinspection GoUnhandledErrorResult
	defer f3.Close()
	infos, err := f3.Readdir(0)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if info.IsDir() && strings.HasPrefix(info.Name(), "GeoLite2-City") {
			return os.Rename(filepath.Join(dest, info.Name()), ddest)
		}
	}
	return nil
}

