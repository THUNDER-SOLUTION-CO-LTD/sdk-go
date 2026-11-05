package thunder

import (
	"bytes"

	"io"
	"os"
	"path/filepath"
)

// Image is a validated upload. Data is read before any quota-spending request.
type Image struct {
	Data        []byte
	ContentType string
	Filename    string
}

// ReadImage reads a caller-owned stream, bounded to maxBytes plus one byte.
// The legacy filename argument is ignored; uploads use slip plus the sniffed extension.
func ReadImage(reader io.Reader, _ string, maxBytes int64) (Image, error) {
	if reader == nil || maxBytes <= 0 || maxBytes == int64(^uint64(0)>>1) {
		return Image{}, inputError("INVALID_ARGUMENT", "image reader and positive size limit are required")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return Image{}, inputError("FILE_NOT_READABLE", "cannot read image")
	}
	if int64(len(data)) > maxBytes {
		return Image{}, inputError("FILE_TOO_LARGE", "image exceeds size limit")
	}
	if len(data) == 0 {
		return Image{}, inputError("EMPTY_FILE", "Image is empty")
	}
	mime := ""
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		mime = "image/png"
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		mime = "image/jpeg"
	case bytes.HasPrefix(data, []byte("GIF8")):
		mime = "image/gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		mime = "image/webp"
	default:
		return Image{}, inputError("UNSUPPORTED_FILE_TYPE", "unsupported image format")
	}
	filename := "slip." + map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/gif": "gif", "image/webp": "webp"}[mime]
	return Image{Data: data, ContentType: mime, Filename: filepath.Base(filename)}, nil
}

// OpenImage reads and closes an image file without exposing its path in errors.
func OpenImage(path string, maxBytes int64) (Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return Image{}, inputError("FILE_NOT_READABLE", "cannot open image file")
	}
	defer file.Close()
	return ReadImage(file, filepath.Base(path), maxBytes)
}

// ImageBytes validates image bytes using the same size and format rules as streams.
func ImageBytes(data []byte, maxBytes int64) (Image, error) {
	return ReadImage(bytes.NewReader(data), "", maxBytes)
}
