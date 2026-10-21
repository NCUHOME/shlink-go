package geoip

import (
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

type Location struct {
	CountryCode string  `json:"countryCode"`
	CountryName string  `json:"countryName"`
	RegionName  string  `json:"regionName"`
	CityName    string  `json:"cityName"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Timezone    string  `json:"timezone"`
}

type Locator struct {
	reader *geoip2.Reader
}

func Open(path string) (*Locator, error) {
	if strings.TrimSpace(path) == "" {
		return &Locator{}, nil
	}
	reader, err := geoip2.Open(path)
	if err != nil {
		return nil, err
	}
	return &Locator{reader: reader}, nil
}

func (l *Locator) Close() error {
	if l == nil || l.reader == nil {
		return nil
	}
	return l.reader.Close()
}

func (l *Locator) Lookup(ip net.IP) *Location {
	if l == nil || l.reader == nil || ip == nil {
		return nil
	}
	city, err := l.reader.City(ip)
	if err != nil {
		return nil
	}
	location := &Location{
		CountryCode: city.Country.IsoCode,
		CountryName: city.Country.Names["en"],
		CityName:    city.City.Names["en"],
		Latitude:    city.Location.Latitude,
		Longitude:   city.Location.Longitude,
		Timezone:    city.Location.TimeZone,
	}
	if len(city.Subdivisions) > 0 {
		location.RegionName = city.Subdivisions[0].Names["en"]
	}
	if location.CountryCode == "" && location.CountryName == "" && location.CityName == "" {
		return nil
	}
	return location
}
