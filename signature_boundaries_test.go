package thunder

import (
	"bytes"
	"errors"
	"testing"
)

func TestSignatureBoundaries(t *testing.T) {
	const limit = 4194304
	cases := []struct {
		name string
		data []byte
		mime string
		code string
	}{
		{"jpeg-min", []byte{0xff, 0xd8, 0xff}, "image/jpeg", ""},
		{"jpeg-short", []byte{0xff, 0xd8}, "", "UNSUPPORTED_FILE_TYPE"},
		{"png-min", []byte("\x89PNG"), "image/png", ""},
		{"png-short", []byte("\x89PN"), "", "UNSUPPORTED_FILE_TYPE"},
		{"gif-min", []byte("GIF8"), "image/gif", ""},
		{"gif-short", []byte("GIF"), "", "UNSUPPORTED_FILE_TYPE"},
		{"webp-min", []byte("RIFFabcdWEBP"), "image/webp", ""},
		{"webp-short", []byte("RIFFabcdWEB"), "", "UNSUPPORTED_FILE_TYPE"},
		{"png-eight", []byte("\x89PNG\r\n\x1a\n"), "image/png", ""},
		{"empty", nil, "", "EMPTY_FILE"},
		{"at-limit", append([]byte("\x89PNG"), make([]byte, limit-4)...), "image/png", ""},
		{"over-limit", append([]byte("\x89PNG"), make([]byte, limit-3)...), "", "FILE_TOO_LARGE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			image, err := ImageBytes(c.data, limit)
			if c.code != "" {
				var input *InputError
				if !errors.As(err, &input) || input.Code != c.code {
					t.Fatalf("expected %s, got %v", c.code, err)
				}
				return
			}
			if err != nil || image.ContentType != c.mime || !bytes.Equal(image.Data, c.data) {
				t.Fatalf("expected %s, got %v %v", c.mime, image.ContentType, err)
			}
		})
	}
}
