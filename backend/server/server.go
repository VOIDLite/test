package server

import (
	"backend/api"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// New creates a new HTTP server.
func New(port string) *http.Server {
	mux := http.NewServeMux()

	// Handle API routes
	mux.HandleFunc("/api/user/login", api.LoginHandler)
	mux.HandleFunc("/api/user/logout", api.LogoutHandler)

	mux.HandleFunc("/api/file/list", api.AuthMiddleware(api.ListFilesHandler))
	mux.HandleFunc("/api/file/upload", api.AuthMiddleware(api.UploadFileHandler))
	mux.HandleFunc("/api/file/new-folder", api.AuthMiddleware(api.NewFolderHandler))
	mux.HandleFunc("/api/file/rename", api.AuthMiddleware(api.RenameFileHandler))
	mux.HandleFunc("/api/file/delete", api.AuthMiddleware(api.DeleteFileHandler))
	mux.HandleFunc("/api/file/move", api.AuthMiddleware(api.MoveFileHandler))
	mux.HandleFunc("/api/file/zip", api.AuthMiddleware(api.ZipFileHandler))

	// Handle static files
	staticServer := http.FileServer(http.Dir("frontend"))
	mux.Handle("/shttps-static-public/", http.StripPrefix("/shttps-static-public/", staticServer))

	// Handle login page
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "frontend/auth/login.html")
	})

	// Handle root and file browser paths
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// a little security check
		if strings.Contains(path, "..") {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// If the path is a directory, serve the file browser's index.html
		// If the path is a file, serve the file.
		// If the path doesn't exist, serve the file browser's index.html
		// to support client-side routing.

		fsPath := filepath.Join("frontend", path)
		stat, err := os.Stat(fsPath)
		if err == nil && !stat.IsDir() {
			http.ServeFile(w, r, fsPath)
			return
		}

		// For any other case, serve the file browser's index.html
		http.ServeFile(w, r, "frontend/file-browser/index.html")
	})

	return &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
}
