package chat

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// resolveImageURLForLLM converts stored image paths to a format that LLM APIs can consume.
// - data: URIs and http(s):// URLs are returned as-is.
// - resource:// references are read through the application resolver and
// converted to base64 data URIs.
func resolveImageURLForLLM(imageURL string) string {
	if strings.HasPrefix(imageURL, "data:") || strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		return imageURL
	}
	if isApplicationStoredImage(imageURL) {
		data := readStoredImageBytes(imageURL)
		if data != nil {
			mime := http.DetectContentType(data)
			return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
		}
	}
	return imageURL
}

// resolveImageURLForOllama converts stored image paths to raw bytes for the Ollama API.
func resolveImageURLForOllama(imageURL string) []byte {
	if strings.HasPrefix(imageURL, "data:") {
		idx := strings.Index(imageURL, ";base64,")
		if idx < 0 {
			return nil
		}
		decoded, err := base64.StdEncoding.DecodeString(imageURL[idx+8:])
		if err != nil {
			return nil
		}
		return decoded
	}
	if isApplicationStoredImage(imageURL) {
		return readStoredImageBytes(imageURL)
	}
	return nil
}

func isApplicationStoredImage(imageURL string) bool {
	return strings.HasPrefix(imageURL, "resource://")
}

// StoredImageResolver reads a resource:// reference from whichever storage
// backend holds it. The application layer sets it at startup; when nil (e.g.
// in tests) stored images cannot be inlined and are passed through unchanged.
var StoredImageResolver func(ref string) ([]byte, bool)

// readStoredImageBytes returns the bytes of a stored image, or nil.
func readStoredImageBytes(ref string) []byte {
	if StoredImageResolver == nil {
		return nil
	}
	data, ok := StoredImageResolver(ref)
	if !ok {
		log.Printf("[image-resolve] failed to read stored image %s", ref)
		return nil
	}
	return data
}

// isMultimodalNotSupportedError checks if an error indicates the model does not
// support multimodal/image input.
func isMultimodalNotSupportedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return (strings.Contains(msg, "multimodal") || strings.Contains(msg, "image") || strings.Contains(msg, "vision")) &&
		(strings.Contains(msg, "not support") || strings.Contains(msg, "unsupported") || strings.Contains(msg, "400"))
}

// stripImagesFromMessages returns a copy of messages with all image data removed.
func stripImagesFromMessages(messages []Message) []Message {
	cleaned := make([]Message, len(messages))
	for i, msg := range messages {
		cleaned[i] = msg
		cleaned[i].Images = nil
	}
	return cleaned
}
