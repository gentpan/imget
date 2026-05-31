// Package mediatype centralizes the image content-type ⇄ extension mapping
// that was previously duplicated across the server, imgpipe and source layers.
package mediatype

import "strings"

// ForExt returns the image content type for a file extension or bare format
// name (e.g. "webp", ".webp", "JPG" all map to the right type). Unknown inputs
// fall back to application/octet-stream.
func ForExt(formatOrExt string) string {
	switch strings.ToLower(strings.TrimPrefix(formatOrExt, ".")) {
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	}
	return "application/octet-stream"
}

// ExtForContentType maps an HTTP Content-Type (parameters tolerated) to a
// canonical file extension including the leading dot, or "" if unrecognized.
func ExtForContentType(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/avif":
		return ".avif"
	}
	return ""
}
