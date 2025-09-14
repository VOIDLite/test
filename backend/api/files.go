package api

import (
	"backend/filesystem"
	"encoding/json"
	"net/http"
	"path/filepath"
)

// ListFilesHandler handles the request for listing files.
func ListFilesHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}
	sortType := r.URL.Query().Get("sort")
	sortReversed := r.URL.Query().Get("sort-reversed") == "true"

	// Basic security check
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	files, err := filesystem.List(cleanPath, sortType, sortReversed)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// UploadFileHandler handles file uploads.
func UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}

	// 32 MB max memory
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle uploaded files
	files := r.MultipartForm.File["files[]"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		filePath := filepath.Join(path, fileHeader.Filename)
		if err := filesystem.Upload(file, filePath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Handle empty directories
	emptyDirs := r.MultipartForm.Value["emptyDirs[]"]
	for _, dir := range emptyDirs {
		dirPath := filepath.Join(path, dir)
		if err := filesystem.CreateFolder(dirPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ZipFileHandler handles zipping files and folders.
func ZipFileHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path := r.FormValue("path")
	filesJSON := r.FormValue("files")

	var files []string
	if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
		http.Error(w, "Invalid files format", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"archive.zip\"")

	if err := filesystem.Zip(files, w, path); err != nil {
		// Can't set http error here because header is already written.
		// Log the error instead.
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// MoveFileHandler handles moving or copying files.
func MoveFileHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Action string   `json:"action"`
		Path   string   `json:"path"`
		Files  []string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, file := range payload.Files {
		if payload.Action == "move" {
			if err := filesystem.Move(file, payload.Path); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else if payload.Action == "copy" {
			if err := filesystem.Copy(file, payload.Path); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteFileHandler handles deleting files or folders.
func DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path  string   `json:"path"`
		Files []string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, file := range payload.Files {
		path := filepath.Join(payload.Path, file)
		if err := filesystem.Delete(path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// RenameFileHandler handles renaming a file or folder.
func RenameFileHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path := r.FormValue("path")
	name := r.FormValue("name")

	if path == "" || name == "" {
		http.Error(w, "Path and name are required", http.StatusBadRequest)
		return
	}

	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, name)

	if err := filesystem.Rename(path, newPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// NewFolderHandler handles creating a new folder.
func NewFolderHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path := r.FormValue("path")
	name := r.FormValue("name")

	if path == "" || name == "" {
		http.Error(w, "Path and name are required", http.StatusBadRequest)
		return
	}

	folderPath := filepath.Join(path, name)
	if err := filesystem.CreateFolder(folderPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
