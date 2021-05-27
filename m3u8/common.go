package m3u8

import (
	"net/url"
	"path/filepath"
)

func UriPrefix(uri string) (string, error) {
	u, err := url.Parse(uri)

	if err != nil {
		return "", err
	}

	pu := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   filepath.Dir(u.Path),
	}

	return pu.String(), nil
}

func UriBase(uri string) (string, error) {
	u, err := url.Parse(uri)

	if err != nil {
		return "", err
	}

	return filepath.Base(u.Path), nil
}
