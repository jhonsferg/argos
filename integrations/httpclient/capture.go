package argoshttpclient

import (
	"bytes"
	"io"
)

// drainAndCapture reads up to max bytes from body, returning them alongside
// a replacement io.ReadCloser that reproduces body's full original content
// (the captured prefix, then whatever was left) - so capturing never
// truncates what the real request/response actually carries. body == nil or
// max <= 0 is a no-op. A read error mid-capture is treated as "nothing
// captured, body left untouched" rather than failing the call over an
// opt-in diagnostic feature.
func drainAndCapture(body io.ReadCloser, max int) ([]byte, io.ReadCloser) {
	if body == nil || max <= 0 {
		return nil, body
	}
	captured, err := io.ReadAll(io.LimitReader(body, int64(max)))
	if err != nil {
		return nil, body
	}
	return captured, splicedReadCloser{Reader: io.MultiReader(bytes.NewReader(captured), body), closer: body}
}

// splicedReadCloser lets a reader that already consumed the first bytes of
// body still be closed correctly by delegating Close to the original body.
type splicedReadCloser struct {
	io.Reader
	closer io.Closer
}

func (s splicedReadCloser) Close() error { return s.closer.Close() }
