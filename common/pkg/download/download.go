package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// FromURL downloads the specified source to a file in tmpdir (OS defaults if
// empty).
func FromURL(ctx context.Context, tmpdir, source string) (string, error) {
	tmp, err := os.CreateTemp(tmpdir, "")
	if err != nil {
		return "", fmt.Errorf("creating temporary download file: %w", err)
	}
	defer tmp.Close()
	succeeded := false
	defer func() {
		if !succeeded {
			os.Remove(tmp.Name())
		}
	}()

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", fmt.Errorf("preparing to download %q: %w", source, err)
	}
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", source, err)
	}
	defer response.Body.Close()

	_, err = io.Copy(tmp, response.Body)
	if err != nil {
		return "", fmt.Errorf("copying %s to %s: %w", source, tmp.Name(), err)
	}

	succeeded = true
	return tmp.Name(), nil
}
