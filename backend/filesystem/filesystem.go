package filesystem

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileInfo struct {
	Name      string `json:"name"`
	Directory bool   `json:"directory"`
	Modified  int64  `json:"modified"` // Unix timestamp in milliseconds
	Length    int64  `json:"length"`
}

// List returns a list of files and directories in a given path.
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
			continue // Skip files that can't be read
		}
		fileInfos = append(fileInfos, FileInfo{
			Name:      info.Name(),
			Directory: info.IsDir(),
			Modified:  info.ModTime().UnixMilli(),
			Length:    info.Size(),
		})
	}

	// Sort the file list
	sort.Slice(fileInfos, func(i, j int) bool {
		// Directories always come first
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
		default: // "default", "gallery"
			result = strings.ToLower(fileInfos[i].Name) < strings.ToLower(fileInfos[j].Name)
		}

		if sortReversed {
			return !result
		}
		return result
	})

	return fileInfos, nil
}

// CreateFolder creates a new folder.
func CreateFolder(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

// Rename renames a file or folder.
func Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// Delete deletes a file or folder.
func Delete(path string) error {
	return os.RemoveAll(path)
}

// Move moves a file or folder.
func Move(source, destination string) error {
	destInfo, err := os.Stat(destination)
	if err == nil && destInfo.IsDir() {
		destination = filepath.Join(destination, filepath.Base(source))
	}
	return os.Rename(source, destination)
}

// Copy copies a file or folder recursively.
func Copy(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		// If destination is a directory, copy inside it.
		// If it doesn't exist, it will be created.
		dst = filepath.Join(dst, filepath.Base(src))
		return copyDir(src, dst)
	}
	// If destination is a directory, copy inside it.
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


// Upload saves a file from a reader to a path.
func Upload(reader io.Reader, path string) error {
	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, reader)
	return err
}

// Zip creates a zip archive from a list of files.
func Zip(files []string, destination io.Writer, rootPath string) error {
	zipWriter := zip.NewWriter(destination)
	defer zipWriter.Close()

	for _, file := range files {
		fullPath := filepath.Join(rootPath, file)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue // Or return error? Frontend seems to expect success.
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
