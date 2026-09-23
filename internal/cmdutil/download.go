package cmdutil

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// DownloadFile saves the file at url to dest. The token is sent only when
// url is on host: a release asset can link to an outside server, and that
// server must not receive it.
func DownloadFile(url, host, token, dest string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if strings.EqualFold(req.URL.Host, host) {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
