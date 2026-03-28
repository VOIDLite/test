package main

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// --- Structs ---

type FileInfo struct {
	Name      string `json:"name"`
	Directory bool   `json:"directory"`
	Modified  int64  `json:"modified"` // Unix timestamp in milliseconds
	Length    int64  `json:"length"`
}

type session struct {
	username string
	expiry   time.Time
}

// --- Globals ---

var (
	sessions = make(map[string]session)
	mutex    = &sync.Mutex{}
)

const (
	hardcodedUsername       = "admin"
	hardcodedHashedPassword = "29e0394f" // FNV-1a hash of "password"
	SessionCookieName       = "session_token"
)

var ErrInvalidCredentials = errors.New("invalid username or password")

// --- Functions from auth package ---

func (s session) isExpired() bool {
	return s.expiry.Before(time.Now())
}

func Login(username, hashedPassword string) (string, error) {
	if username == hardcodedUsername && hashedPassword == hardcodedHashedPassword {
		sessionToken := make([]byte, 32)
		if _, err := rand.Read(sessionToken); err != nil {
			return "", err
		}
		tokenString := hex.EncodeToString(sessionToken)
		mutex.Lock()
		sessions[tokenString] = session{
			username: username,
			expiry:   time.Now().Add(24 * time.Hour),
		}
		mutex.Unlock()
		return tokenString, nil
	}
	return "", ErrInvalidCredentials
}

func Logout(token string) {
	mutex.Lock()
	delete(sessions, token)
	mutex.Unlock()
}

func IsAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return false
	}
	sessionToken := cookie.Value
	mutex.Lock()
	userSession, exists := sessions[sessionToken]
	mutex.Unlock()
	if !exists {
		return false
	}
	if userSession.isExpired() {
		Logout(sessionToken)
		return false
	}
	return true
}

func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
	})
}

// --- Functions from filesystem package ---

func List(rootPath string, sortType string, sortReversed bool) ([]FileInfo, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	var fileInfos []FileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, FileInfo{
			Name:      info.Name(),
			Directory: info.IsDir(),
			Modified:  info.ModTime().UnixMilli(),
			Length:    info.Size(),
		})
	}
	sort.Slice(fileInfos, func(i, j int) bool {
		if fileInfos[i].Directory != fileInfos[j].Directory {
			return fileInfos[i].Directory
		}
		var result bool
		switch sortType {
		case "modified":
			result = fileInfos[i].Modified > fileInfos[j].Modified
		case "size":
			if fileInfos[i].Directory {
				result = strings.ToLower(fileInfos[i].Name) < strings.ToLower(fileInfos[j].Name)
			} else {
				result = fileInfos[i].Length > fileInfos[j].Length
			}
		default:
			result = strings.ToLower(fileInfos[i].Name) < strings.ToLower(fileInfos[j].Name)
		}
		if sortReversed {
			return !result
		}
		return result
	})
	return fileInfos, nil
}

func CreateFolder(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

func Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func Delete(path string) error {
	return os.RemoveAll(path)
}

func Move(source, destination string) error {
	destInfo, err := os.Stat(destination)
	if err == nil && destInfo.IsDir() {
		destination = filepath.Join(destination, filepath.Base(source))
	}
	return os.Rename(source, destination)
}

func Copy(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		dst = filepath.Join(dst, filepath.Base(src))
		return copyDir(src, dst)
	}
	destInfo, err := os.Stat(dst)
	if err == nil && destInfo.IsDir() {
		dst = filepath.Join(dst, filepath.Base(src))
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func Upload(reader io.Reader, path string) error {
	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, reader)
	return err
}

func Zip(files []string, destination io.Writer, rootPath string) error {
	zipWriter := zip.NewWriter(destination)
	defer zipWriter.Close()
	for _, file := range files {
		fullPath := filepath.Join(rootPath, file)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.IsDir() {
			err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				header, err := zip.FileInfoHeader(info)
				if err != nil {
					return err
				}
				relPath, err := filepath.Rel(rootPath, path)
				if err != nil {
					return err
				}
				header.Name = relPath
				if info.IsDir() {
					header.Name += "/"
				} else {
					header.Method = zip.Deflate
				}
				writer, err := zipWriter.CreateHeader(header)
				if err != nil {
					return err
				}
				if !info.IsDir() {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()
					_, err = io.Copy(writer, file)
				}
				return err
			})
			if err != nil {
				return err
			}
		} else {
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = file
			header.Method = zip.Deflate
			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}
			fileData, err := os.Open(fullPath)
			if err != nil {
				return err
			}
			defer fileData.Close()
			_, err = io.Copy(writer, fileData)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Functions from api package ---

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	sessionToken, err := Login(username, password)
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	SetSessionCookie(w, sessionToken)
	w.WriteHeader(http.StatusOK)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	Logout(cookie.Value)
	ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func ListFilesHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}
	sortType := r.URL.Query().Get("sort")
	sortReversed := r.URL.Query().Get("sort-reversed") == "true"
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	files, err := List(cleanPath, sortType, sortReversed)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	files := r.MultipartForm.File["files[]"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		filePath := filepath.Join(path, fileHeader.Filename)
		if err := Upload(file, filePath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	emptyDirs := r.MultipartForm.Value["emptyDirs[]"]
	for _, dir := range emptyDirs {
		dirPath := filepath.Join(path, dir)
		if err := CreateFolder(dirPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

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
	if err := Zip(files, w, path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

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
			if err := Move(file, payload.Path); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else if payload.Action == "copy" {
			if err := Copy(file, payload.Path); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

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
		if err := Delete(path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

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
	if err := Rename(path, newPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
	if err := CreateFolder(folderPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Main function ---

func main() {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/user/login", LoginHandler)
	mux.HandleFunc("/api/user/logout", LogoutHandler)
	mux.HandleFunc("/api/file/list", AuthMiddleware(ListFilesHandler))
	mux.HandleFunc("/api/file/upload", AuthMiddleware(UploadFileHandler))
	mux.HandleFunc("/api/file/new-folder", AuthMiddleware(NewFolderHandler))
	mux.HandleFunc("/api/file/rename", AuthMiddleware(RenameFileHandler))
	mux.HandleFunc("/api/file/delete", AuthMiddleware(DeleteFileHandler))
	mux.HandleFunc("/api/file/move", AuthMiddleware(MoveFileHandler))
	mux.HandleFunc("/api/file/zip", AuthMiddleware(ZipFileHandler))

	// Static files
	staticServer := http.FileServer(http.Dir("../frontend"))
	mux.Handle("/shttps-static-public/", http.StripPrefix("/shttps-static-public/", staticServer))
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../frontend/auth/login.html")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "..") {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		fsPath := filepath.Join("../frontend", path)
		stat, err := os.Stat(fsPath)
		if err == nil && !stat.IsDir() {
			http.ServeFile(w, r, fsPath)
			return
		}
		http.ServeFile(w, r, "../frontend/file-browser/index.html")
	})

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
