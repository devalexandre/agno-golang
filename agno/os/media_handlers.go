package os

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
)

// processImageUploads processes uploaded image files and returns Image objects
func processImageUploads(files []*multipart.FileHeader) ([]*agent.Image, error) {
	var images []*agent.Image

	for _, fileHeader := range files {
		// Validate file type
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if !isImageFormat(ext) {
			return nil, fmt.Errorf("unsupported image format: %s (supported: .png, .jpg, .jpeg, .webp, .gif)", ext)
		}

		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open image file %s: %w", fileHeader.Filename, err)
		}
		defer file.Close()

		// Read file content
		content := make([]byte, fileHeader.Size)
		_, err = file.Read(content)
		if err != nil {
			return nil, fmt.Errorf("failed to read image file %s: %w", fileHeader.Filename, err)
		}

		// Create Image object
		image := &agent.Image{
			ID:       generateID("image"),
			Data:     content,
			MimeType: getMimeTypeFromExtension(ext),
		}

		images = append(images, image)
	}

	return images, nil
}

// processAudioUploads processes uploaded audio files and returns Audio objects
func processAudioUploads(files []*multipart.FileHeader) ([]*agent.Audio, error) {
	var audioList []*agent.Audio

	for _, fileHeader := range files {
		// Validate file type
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if !isAudioFormat(ext) {
			return nil, fmt.Errorf("unsupported audio format: %s (supported: .wav, .mp3, .ogg, .m4a, .flac)", ext)
		}

		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open audio file %s: %w", fileHeader.Filename, err)
		}
		defer file.Close()

		// Read file content
		content := make([]byte, fileHeader.Size)
		_, err = file.Read(content)
		if err != nil {
			return nil, fmt.Errorf("failed to read audio file %s: %w", fileHeader.Filename, err)
		}

		// Create Audio object
		audio := &agent.Audio{
			ID:       generateID("audio"),
			Data:     content,
			MimeType: getMimeTypeFromExtension(ext),
		}

		audioList = append(audioList, audio)
	}

	return audioList, nil
}

// processVideoUploads processes uploaded video files and returns Video objects
func processVideoUploads(files []*multipart.FileHeader) ([]*agent.Video, error) {
	var videos []*agent.Video

	for _, fileHeader := range files {
		// Validate file type
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if !isVideoFormat(ext) {
			return nil, fmt.Errorf("unsupported video format: %s (supported: .mp4, .webm, .avi, .mov, .mkv)", ext)
		}

		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open video file %s: %w", fileHeader.Filename, err)
		}
		defer file.Close()

		// Read file content
		content := make([]byte, fileHeader.Size)
		_, err = file.Read(content)
		if err != nil {
			return nil, fmt.Errorf("failed to read video file %s: %w", fileHeader.Filename, err)
		}

		// Create Video object
		video := &agent.Video{
			ID:       generateID("video"),
			Data:     content,
			MimeType: getMimeTypeFromExtension(ext),
		}

		videos = append(videos, video)
	}

	return videos, nil
}

// processFileUploads processes uploaded document files and returns File objects
func processFileUploads(files []*multipart.FileHeader) ([]*agent.File, error) {
	var fileList []*agent.File

	for _, fileHeader := range files {
		// Validate file type
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if !isDocumentFormat(ext) {
			return nil, fmt.Errorf("unsupported file format: %s (supported: .pdf, .csv, .docx, .txt, .json, .md)", ext)
		}

		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", fileHeader.Filename, err)
		}
		defer file.Close()

		// Read file content
		content := make([]byte, fileHeader.Size)
		_, err = file.Read(content)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", fileHeader.Filename, err)
		}

		// Create File object
		docFile := &agent.File{
			ID:       generateID("file"),
			Name:     fileHeader.Filename,
			Data:     content,
			MimeType: getMimeTypeFromExtension(ext),
		}

		fileList = append(fileList, docFile)
	}

	return fileList, nil
}

// isImageFormat checks if the file extension is a supported image format
func isImageFormat(ext string) bool {
	supportedFormats := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".webp": true,
		".gif":  true,
	}
	return supportedFormats[ext]
}

// isAudioFormat checks if the file extension is a supported audio format
func isAudioFormat(ext string) bool {
	supportedFormats := map[string]bool{
		".wav":  true,
		".mp3":  true,
		".ogg":  true,
		".m4a":  true,
		".flac": true,
	}
	return supportedFormats[ext]
}

// isVideoFormat checks if the file extension is a supported video format
func isVideoFormat(ext string) bool {
	supportedFormats := map[string]bool{
		".mp4":  true,
		".webm": true,
		".avi":  true,
		".mov":  true,
		".mkv":  true,
	}
	return supportedFormats[ext]
}

// isDocumentFormat checks if the file extension is a supported document format
func isDocumentFormat(ext string) bool {
	supportedFormats := map[string]bool{
		".pdf":  true,
		".csv":  true,
		".docx": true,
		".txt":  true,
		".json": true,
		".md":   true,
		".html": true,
		".xml":  true,
	}
	return supportedFormats[ext]
}

// getMimeTypeFromExtension returns the MIME type for a given file extension
func getMimeTypeFromExtension(ext string) string {
	mimeTypes := map[string]string{
		// Images
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".webp": "image/webp",
		".gif":  "image/gif",

		// Audio
		".wav":  "audio/wav",
		".mp3":  "audio/mpeg",
		".ogg":  "audio/ogg",
		".m4a":  "audio/mp4",
		".flac": "audio/flac",

		// Video
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".mkv":  "video/x-matroska",

		// Documents
		".pdf":  "application/pdf",
		".csv":  "text/csv",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".txt":  "text/plain",
		".json": "application/json",
		".md":   "text/markdown",
		".html": "text/html",
		".xml":  "application/xml",
	}

	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}
	return "application/octet-stream"
}
