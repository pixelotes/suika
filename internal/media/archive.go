package media

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nwaples/rardecode/v2"
)

// ListPages returns sorted image filenames without extracting data.
func ListPages(archivePath string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))
	switch ext {
	case ".cbz", ".zip":
		return listZipImages(archivePath)
	case ".cbr", ".rar":
		return listRarImages(archivePath)
	default:
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}
}

// GetPages is an alias for ListPages.
func GetPages(archivePath string) ([]string, error) {
	return ListPages(archivePath)
}

// GetPage extracts a specific page by index.
func GetPage(archivePath string, pageIndex int) ([]byte, string, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))
	switch ext {
	case ".cbz", ".zip":
		return getZipPage(archivePath, pageIndex)
	case ".cbr", ".rar":
		return getRarPage(archivePath, pageIndex)
	default:
		return nil, "", fmt.Errorf("unsupported format: %s", ext)
	}
}

// ExtractCover extracts the first image from an archive.
func ExtractCover(archivePath string) ([]byte, string, error) {
	return GetPage(archivePath, 0)
}

// ReadFileFromArchive reads a specific file by name from an archive.
func ReadFileFromArchive(archivePath, fileName string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))
	switch ext {
	case ".cbz", ".zip":
		return readZipFile(archivePath, fileName)
	case ".cbr", ".rar":
		return readRarFile(archivePath, fileName)
	default:
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}
}

// --- ZIP (random access) ---

func listZipImages(path string) ([]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var names []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if imageExts[strings.ToLower(filepath.Ext(f.Name))] {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func getZipPage(path string, pageIndex int) ([]byte, string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, "", err
	}
	defer r.Close()

	var images []*zip.File
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if imageExts[strings.ToLower(filepath.Ext(f.Name))] {
			images = append(images, f)
		}
	}
	sort.Slice(images, func(i, j int) bool {
		return images[i].Name < images[j].Name
	})

	if pageIndex < 0 || pageIndex >= len(images) {
		return nil, "", fmt.Errorf("page %d out of range (0-%d)", pageIndex, len(images)-1)
	}

	rc, err := images[pageIndex].Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", err
	}
	return data, contentTypeForExt(filepath.Ext(images[pageIndex].Name)), nil
}

func readZipFile(path, fileName string) ([]byte, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.EqualFold(filepath.Base(f.Name), fileName) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file not found: %s", fileName)
}

// --- RAR (sequential access) ---

func listRarImages(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := rardecode.NewReader(f)
	if err != nil {
		return nil, err
	}

	var names []string
	for {
		header, err := r.Next()
		if err != nil {
			break
		}
		if !header.IsDir && imageExts[strings.ToLower(filepath.Ext(header.Name))] {
			names = append(names, header.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func getRarPage(path string, pageIndex int) ([]byte, string, error) {
	// First get sorted page names
	names, err := listRarImages(path)
	if err != nil {
		return nil, "", err
	}
	if pageIndex < 0 || pageIndex >= len(names) {
		return nil, "", fmt.Errorf("page %d out of range (0-%d)", pageIndex, len(names)-1)
	}
	targetName := names[pageIndex]

	// Second pass: find and extract the target
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	r, err := rardecode.NewReader(f)
	if err != nil {
		return nil, "", err
	}

	for {
		header, err := r.Next()
		if err != nil {
			break
		}
		if header.Name == targetName {
			data, err := io.ReadAll(r)
			if err != nil {
				return nil, "", err
			}
			return data, contentTypeForExt(filepath.Ext(targetName)), nil
		}
	}
	return nil, "", fmt.Errorf("page not found: %s", targetName)
}

func readRarFile(path, fileName string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := rardecode.NewReader(f)
	if err != nil {
		return nil, err
	}

	for {
		header, err := r.Next()
		if err != nil {
			break
		}
		if strings.EqualFold(filepath.Base(header.Name), fileName) {
			return io.ReadAll(r)
		}
	}
	return nil, fmt.Errorf("file not found: %s", fileName)
}
