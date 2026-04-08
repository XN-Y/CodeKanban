package websession

import (
	"fmt"
	"mime"
	"path/filepath"
	"regexp"
	"strings"
)

var managedImagePlaceholderTokenPattern = regexp.MustCompile(`\[Image #\d+\]`)
var managedImagePlaceholderLinePattern = regexp.MustCompile(`(?m)(^|\n)\s*(?:\[Image #\d+\]\s*)+(?:\n|$)`)
var trailingLineWhitespacePattern = regexp.MustCompile(`[ \t]+\n`)
var repeatedBlankLinePattern = regexp.MustCompile(`\n{3,}`)

func buildManagedImagePlaceholder(index int) string {
	return fmt.Sprintf("[Image #%d]", index)
}

func buildManagedImagePlaceholderLine(count int) string {
	if count <= 0 {
		return ""
	}

	parts := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		parts = append(parts, buildManagedImagePlaceholder(index))
	}
	return strings.Join(parts, " ")
}

func stripManagedImagePlaceholders(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	if normalized == "" {
		return ""
	}

	normalized = managedImagePlaceholderLinePattern.ReplaceAllString(normalized, "\n")
	lines := strings.Split(normalized, "\n")
	for index, line := range lines {
		if managedImagePlaceholderTokenPattern.MatchString(line) {
			line = managedImagePlaceholderTokenPattern.ReplaceAllString(line, " ")
			line = strings.Join(strings.Fields(line), " ")
		} else {
			line = strings.TrimRight(line, " \t")
		}
		lines[index] = line
	}
	normalized = strings.Join(lines, "\n")
	normalized = trailingLineWhitespacePattern.ReplaceAllString(normalized, "\n")
	normalized = repeatedBlankLinePattern.ReplaceAllString(normalized, "\n\n")
	return strings.TrimSpace(normalized)
}

func composeUserMessageText(text string, attachmentCount int) string {
	baseText := stripManagedImagePlaceholders(text)
	placeholderLine := buildManagedImagePlaceholderLine(attachmentCount)
	switch {
	case placeholderLine == "":
		return baseText
	case baseText == "":
		return placeholderLine
	default:
		return baseText + "\n\n" + placeholderLine
	}
}

func normalizeAttachmentDisplayName(name string, index int) string {
	if strings.TrimSpace(name) == "" {
		return fmt.Sprintf("image %d", index)
	}

	trimmed := strings.TrimSpace(filepath.Base(name))
	if trimmed == "" || isGenericImageAttachmentName(trimmed) {
		return fmt.Sprintf("image %d", index)
	}
	return trimmed
}

func isGenericImageAttachmentName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(filepath.Base(name)))
	if normalized == "" {
		return true
	}

	stem := strings.TrimSuffix(normalized, filepath.Ext(normalized))
	switch stem {
	case "image", "blob", "clipboard-image":
		return true
	}

	return strings.HasPrefix(stem, "pasted-image-")
}

func defaultAttachmentStoredName(contentType string) string {
	ext := attachmentExtensionFromMime(contentType)
	if ext == "" {
		return "image"
	}
	return "image" + ext
}

func attachmentExtensionFromMime(contentType string) string {
	normalized := strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch normalized {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/svg+xml":
		return ".svg"
	case "image/tiff":
		return ".tiff"
	}

	extensions, err := mime.ExtensionsByType(normalized)
	if err != nil || len(extensions) == 0 {
		return ""
	}
	return extensions[0]
}
