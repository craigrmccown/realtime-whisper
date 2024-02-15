package utility

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

func DoMultipartUpload(ctx context.Context, client *http.Client, url, name string, f io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := func() error {
		defer w.Close()

		fw, err := w.CreateFormFile("file", name)
		if err != nil {
			return fmt.Errorf("failed to create multipart file writer: %w", err)
		}

		if _, err = io.Copy(fw, f); err != nil {
			return fmt.Errorf("failed to write file to multipart form: %w", err)
		}

		return nil
	}(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	rsp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	b, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response from server not OK: %d, %s", rsp.StatusCode, b)
	}

	return b, nil
}
