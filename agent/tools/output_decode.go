package tools

import (
	"bytes"
	"runtime"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// decodeOutput converts command output bytes to a valid UTF-8 string.
// On Windows the system code page is often GBK/GB18030, so npm/tsc output
// arrives as GBK-encoded bytes. We attempt GBK decoding first; if it
// produces valid UTF-8 we return it, otherwise fall back to the raw string.
// On non-Windows platforms the output is assumed to be UTF-8 already.
func decodeOutput(b []byte) string {
	if runtime.GOOS != "windows" {
		return string(b)
	}

	// Try GBK → UTF-8
	reader := transform.NewReader(bytes.NewReader(b), simplifiedchinese.GBK.NewDecoder())
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err == nil {
		return buf.String()
	}
	// Fallback: raw string (may contain replacement chars but won't crash)
	return string(b)
}
