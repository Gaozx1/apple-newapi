package helper

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
)

// DimensionsFromBase64Image reads the pixel width/height from the header of a
// base64-encoded PNG, JPEG or WebP image. Only the first few hundred bytes are
// decoded, so this is cheap enough to run during billing settlement. ok is
// false for unsupported formats or truncated data.
func DimensionsFromBase64Image(b64 string) (width, height int, ok bool) {
	if b64 == "" {
		return 0, 0, false
	}
	// Strip an optional data URL prefix and any whitespace/newlines that
	// clients sometimes insert into long base64 payloads.
	if idx := strings.Index(b64, ";base64,"); idx != -1 {
		b64 = b64[idx+len(";base64,"):]
	}
	b64 = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' {
			return -1
		}
		return r
	}, b64)
	if len(b64) < 24 {
		return 0, 0, false
	}
	raw, err := base64.StdEncoding.DecodeString(b64[:min(len(b64), 4096)])
	if err != nil || len(raw) < 8 {
		return 0, 0, false
	}
	switch {
	case hasPrefix(raw, "\x89PNG\r\n\x1a\n"):
		return pngDimensions(raw)
	case hasPrefix(raw, "\xff\xd8"):
		return jpegDimensions(raw)
	case hasPrefix(raw, "RIFF") && len(raw) >= 12 && string(raw[8:12]) == "WEBP":
		return webpDimensions(raw)
	}
	return 0, 0, false
}

func hasPrefix(data []byte, prefix string) bool {
	return len(data) >= len(prefix) && string(data[:len(prefix)]) == prefix
}

func pngDimensions(data []byte) (int, int, bool) {
	// Signature (8) + IHDR length/type (8) + width (4) + height (4) = 24.
	if len(data) < 24 || string(data[12:16]) != "IHDR" {
		return 0, 0, false
	}
	w := int(binary.BigEndian.Uint32(data[16:20]))
	h := int(binary.BigEndian.Uint32(data[20:24]))
	if w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

func jpegDimensions(data []byte) (int, int, bool) {
	pos := 2 // skip the SOI marker
	for pos+9 < len(data) {
		if data[pos] != 0xFF {
			pos++
			continue
		}
		marker := data[pos+1]
		// Standalone markers without a length payload.
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD9) {
			pos += 2
			continue
		}
		if pos+4 > len(data) {
			return 0, 0, false
		}
		segLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		// SOF0-SOF15 except DHT (C4), JPG (C8), DAC (CC) carry dimensions.
		if marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC {
			if pos+9 > len(data) {
				return 0, 0, false
			}
			h := int(binary.BigEndian.Uint16(data[pos+5 : pos+7]))
			w := int(binary.BigEndian.Uint16(data[pos+7 : pos+9]))
			if w <= 0 || h <= 0 {
				return 0, 0, false
			}
			return w, h, true
		}
		pos += 2 + segLen
	}
	return 0, 0, false
}

func webpDimensions(data []byte) (int, int, bool) {
	if len(data) < 30 {
		return 0, 0, false
	}
	chunk := string(data[12:16])
	switch chunk {
	case "VP8 ":
		// Lossy: frame tag (3) + start code (3) + dims at offset 26.
		if len(data) < 30 || data[23] != 0x9d || data[24] != 0x01 || data[25] != 0x2a {
			return 0, 0, false
		}
		w := int(binary.LittleEndian.Uint16(data[26:28]) & 0x3FFF)
		h := int(binary.LittleEndian.Uint16(data[28:30]) & 0x3FFF)
		if w <= 0 || h <= 0 {
			return 0, 0, false
		}
		return w, h, true
	case "VP8L":
		// Lossless: signature byte 0x2F then 14-bit width-1 / height-1.
		if len(data) < 25 || data[20] != 0x2F {
			return 0, 0, false
		}
		bits := binary.LittleEndian.Uint32(data[21:25])
		w := int(bits&0x3FFF) + 1
		h := int((bits>>14)&0x3FFF) + 1
		if w <= 0 || h <= 0 {
			return 0, 0, false
		}
		return w, h, true
	case "VP8X":
		// Extended: 24-bit canvas size minus one at offsets 24 and 27.
		if len(data) < 30 {
			return 0, 0, false
		}
		w := int(uint32(data[24])|uint32(data[25])<<8|uint32(data[26])<<16) + 1
		h := int(uint32(data[27])|uint32(data[28])<<8|uint32(data[29])<<16) + 1
		if w <= 0 || h <= 0 {
			return 0, 0, false
		}
		return w, h, true
	}
	return 0, 0, false
}
