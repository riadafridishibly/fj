package cmdutil

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/riadafridishibly/fj/internal/debug"
)

// DownloadFile saves the file at rawURL to dest. The token goes only to
// host, over https unless FJ_INSECURE is set, on the first request and on
// every redirect: a release asset can link to an outside server, and host
// can redirect to another port or subdomain, such as object storage.
func DownloadFile(rawURL, host, token, dest string) error {
	onHost := func(u *url.URL) bool {
		secure := u.Scheme == "https" || u.Scheme == "http" && os.Getenv("FJ_INSECURE") != ""
		return secure && strings.EqualFold(u.Host, host)
	}
	client := debug.WrapClient(nil)
	// Go keeps Authorization on a redirect to the same hostname on any port
	// or scheme, and to its subdomains.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if !onHost(req.URL) {
			req.Header.Del("Authorization")
		}
		return nil
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return err
	}
	if onHost(req.URL) {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := client.Do(req)
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
	_, err = io.Copy(f, resp.Body)
	// A write error can surface only at Close, and a partial file must not
	// be left to look like a finished download.
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dest)
	}
	return err
}
