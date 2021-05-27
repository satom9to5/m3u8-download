package m3u8

import (
	"io"
	"net/http"
	"os"
)

func Download(url string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func RetryDownload(url string, filepath string) error {
	var err error
	retry := DownloadRetry
	for retry > 0 {
		if err = Download(url, filepath); err == nil {
			return nil
		}

		retry--
	}

	return err
}
