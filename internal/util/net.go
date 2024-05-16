package util

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

func GetHttpFile(ctx context.Context, uri string) ([]byte, error) {
	reader, err := GetHttpFileReader(ctx, uri)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading file from url: %s", uri)
	}

	return data, nil
}

func GetHttpFileReader(ctx context.Context, uri string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed creating request from url: %s", uri)
	}
	client := http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading file from url: %s", uri)
	}
	return resp.Body, nil
}
