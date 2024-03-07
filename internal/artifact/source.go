package artifact

import "io"

// Source implements io.ReadCloser and is a source of a particular artifact.
type Source struct {
	io.ReadCloser
}

// NopSourceCloser returns r as a Source where the Close() method nops.
func NopSourceCloser(r io.Reader) Source {
	return Source{io.NopCloser(r)}
}
