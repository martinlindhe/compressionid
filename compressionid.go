package compressionid

import (
	"bytes"
	"compress/flate"
	"compress/lzw"
	"compress/zlib"
	"fmt"
	"io"
	"os"

	"github.com/pierrec/lz4/v4"
	"github.com/rasky/go-lzo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitLogging() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
}

type CompressionKind int

const (
	ZLib CompressionKind = iota
	Deflate
	LZO1X
	LZ4
	LZW_LSB8 // LSB, 8-bit
)

func (k CompressionKind) String() string {
	switch k {
	case ZLib:
		return "ZLib"
	case Deflate:
		return "Deflate"
	case LZO1X:
		return "LZO1x"
	case LZ4:
		return "LZ4"
	case LZW_LSB8:
		return "LZW-LSB-8"
	default:
		panic(k)
	}
}

func TryExtract(r io.Reader) (CompressionKind, []byte, error) {

	data, err := io.ReadAll(r)
	if err != nil {
		return 0, nil, err
	}

	var b bytes.Buffer
	// ZLIB
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err == nil {
		defer reader.Close()
		_, err = io.Copy(&b, reader)
		if err == nil {
			return ZLib, b.Bytes(), nil
		}
		log.Error().Err(err).Msgf("ZLIB extraction failed")
	} else {
		log.Error().Err(err).Msgf("ZLIB reading failed")
	}

	// DEFLATE
	reader = flate.NewReader(bytes.NewReader(data))
	defer reader.Close()
	_, err = io.Copy(&b, reader)
	if err == nil {
		return Deflate, b.Bytes(), nil
	}
	log.Error().Err(err).Msgf("DEFLATE extraction failed")

	// LZO1X
	expanded, err := lzo.Decompress1X(bytes.NewReader(data), 0, 0)
	if err == nil {
		log.Info().Msgf("Detected LZO1X compression")
		return LZO1X, expanded, nil
	}
	log.Error().Err(err).Msgf("LZO extraction failed")

	// LZ4
	expanded = make([]byte, 1024*1024) // XXX have a "known" expanded size value ready from format parsing
	n, err := lz4.UncompressBlock(data, expanded)
	if err == nil {
		log.Info().Msgf("Detected LZ4 compression")
		return LZ4, expanded[0:n], nil
	}
	log.Error().Err(err).Msgf("LZ4 extraction failed")

	// LZW
	dec := lzw.NewReader(bytes.NewReader(data), lzw.LSB, 8)
	output := make([]byte, 1024*1024) // XXX have a "known" expanded size value ready from format parsing
	count, err := dec.Read(output)
	if err == nil {
		fmt.Println("read", count, "bytes")
		fmt.Printf("output: %#v\n", string(output[:count]))
		log.Info().Msgf("Detected LZW compression")
		return LZW_LSB8, output[:count], nil
	}
	log.Error().Err(err).Msgf("LZW extraction failed")

	// lzfse  - used by apple in xcode?

	return 0, nil, fmt.Errorf("no compression recognized")
}
