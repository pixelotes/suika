package media

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

var archiveExts = map[string]bool{
	".cbz": true, ".cbr": true, ".zip": true, ".rar": true,
}

type BrowseItem struct {
	Name         string    `json:"name"`
	FriendlyName string    `json:"friendly_name,omitempty"`
	Path         string    `json:"path"`
	IsDir        bool      `json:"is_dir"`
	IsArchive    bool      `json:"is_archive"`
	Icon         string    `json:"icon,omitempty"`
	PageCount    int       `json:"page_count,omitempty"`
	Metadata     *ComicInfo `json:"metadata,omitempty"`
}

type BrowseResponse struct {
	Items         []BrowseItem `json:"items"`
	CurrentFolder *FolderInfo  `json:"current_folder,omitempty"`
}

type FolderInfo struct {
	Name             string `json:"name"`
	Poster           string `json:"poster,omitempty"`
	ReadingDirection string `json:"reading_direction,omitempty"`
}

func EncodePath(path string) string {
	return base64.URLEncoding.EncodeToString([]byte(path))
}

func DecodePath(encoded string) (string, error) {
	b, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Browse(dirPath string) (*BrowseResponse, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var items []BrowseItem
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(dirPath, name)
		ext := strings.ToLower(filepath.Ext(name))

		if entry.IsDir() {
			item := BrowseItem{
				Name:  name,
				Path:  fullPath,
				IsDir: true,
			}
			if cover := findCover(fullPath); cover != "" {
				item.Icon = "/api/v1/images/" + EncodePath(cover)
			}
			items = append(items, item)
		} else if archiveExts[ext] {
			item := BrowseItem{
				Name:      name,
				Path:      fullPath,
				IsDir:     false,
				IsArchive: true,
			}
			if hasImages(fullPath) {
				item.Icon = fmt.Sprintf("/api/v1/archive-cover/%s", EncodePath(fullPath))
			}
			if count, err := countPages(fullPath); err == nil {
				item.PageCount = count
			}
			if ci, err := ReadComicInfo(fullPath); err == nil {
				item.Metadata = ci
			}
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return naturalLess(items[i].Name, items[j].Name)
	})

	folder := &FolderInfo{Name: filepath.Base(dirPath)}
	if cover := findCover(dirPath); cover != "" {
		folder.Poster = "/api/v1/images/" + EncodePath(cover)
	}

	return &BrowseResponse{Items: items, CurrentFolder: folder}, nil
}

func findCover(dirPath string) string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}

	priorities := []string{"cover", "poster", "folder"}
	for _, prefix := range priorities {
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			ext := filepath.Ext(name)
			base := strings.TrimSuffix(name, ext)
			if base == prefix && imageExts[ext] {
				return filepath.Join(dirPath, entry.Name())
			}
		}
	}

	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if imageExts[ext] && !entry.IsDir() {
			return filepath.Join(dirPath, entry.Name())
		}
	}

	return ""
}

func hasImages(archivePath string) bool {
	pages, err := ListPages(archivePath)
	return err == nil && len(pages) > 0
}

func countPages(archivePath string) (int, error) {
	pages, err := ListPages(archivePath)
	if err != nil {
		return 0, err
	}
	return len(pages), nil
}

func contentTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

// GetSiblings returns the previous and next archive files in the same directory.
func GetSiblings(archivePath string) (prev, next string) {
	dir := filepath.Dir(archivePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", ""
	}

	var archives []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if archiveExts[ext] {
			archives = append(archives, filepath.Join(dir, entry.Name()))
		}
	}

	sort.Slice(archives, func(i, j int) bool {
		return naturalLess(filepath.Base(archives[i]), filepath.Base(archives[j]))
	})

	for i, a := range archives {
		if a == archivePath {
			if i > 0 {
				prev = archives[i-1]
			}
			if i < len(archives)-1 {
				next = archives[i+1]
			}
			return
		}
	}
	return "", ""
}

func naturalLess(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	ia, ib := 0, 0
	for ia < len(la) && ib < len(lb) {
		ca, cb := la[ia], lb[ib]
		if ca >= '0' && ca <= '9' && cb >= '0' && cb <= '9' {
			na, nb := 0, 0
			for ia < len(la) && la[ia] >= '0' && la[ia] <= '9' {
				na = na*10 + int(la[ia]-'0')
				ia++
			}
			for ib < len(lb) && lb[ib] >= '0' && lb[ib] <= '9' {
				nb = nb*10 + int(lb[ib]-'0')
				ib++
			}
			if na != nb {
				return na < nb
			}
		} else {
			if ca != cb {
				return ca < cb
			}
			ia++
			ib++
		}
	}
	return len(la) < len(lb)
}
