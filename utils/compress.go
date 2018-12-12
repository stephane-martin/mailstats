package utils

import (
	"github.com/pierrec/lz4"
	"github.com/tinylib/msgp/msgp"
	"io"
)

func Compress(w io.Writer, msg msgp.Encodable) error {
	writer := lz4.NewWriter(w)
	writer.Header = lz4.Header{
		CompressionLevel: 0,
	}
	return Autoclose(writer, func() error {
		return msgp.Encode(writer, msg)
	})
}

func Decompress(r io.Reader, msg msgp.Decodable) error {
	lz4Reader := lz4.NewReader(r)
	return msgp.Decode(lz4Reader, msg)
}
