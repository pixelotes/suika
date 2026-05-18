package media

import (
	"encoding/xml"
)

// ComicInfo represents metadata from a ComicInfo.xml file inside CBZ/CBR archives.
// See: https://anansi-project.github.io/docs/comicinfo/documentation
type ComicInfo struct {
	Title      string `xml:"Title" json:"title,omitempty"`
	Series     string `xml:"Series" json:"series,omitempty"`
	Number     string `xml:"Number" json:"number,omitempty"`
	Volume     int    `xml:"Volume" json:"volume,omitempty"`
	Summary    string `xml:"Summary" json:"summary,omitempty"`
	Year       int    `xml:"Year" json:"year,omitempty"`
	Writer     string `xml:"Writer" json:"writer,omitempty"`
	Penciller  string `xml:"Penciller" json:"penciller,omitempty"`
	Publisher  string `xml:"Publisher" json:"publisher,omitempty"`
	Genre      string `xml:"Genre" json:"genre,omitempty"`
	PageCount  int    `xml:"PageCount" json:"page_count,omitempty"`
	LanguageISO string `xml:"LanguageISO" json:"language,omitempty"`
	Manga      string `xml:"Manga" json:"manga,omitempty"` // "Yes", "No", "YesAndRightToLeft"
}

// ReadComicInfo extracts and parses ComicInfo.xml from an archive.
func ReadComicInfo(archivePath string) (*ComicInfo, error) {
	data, err := ReadFileFromArchive(archivePath, "ComicInfo.xml")
	if err != nil {
		return nil, err
	}

	var ci ComicInfo
	if err := xml.Unmarshal(data, &ci); err != nil {
		return nil, err
	}
	return &ci, nil
}
