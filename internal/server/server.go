package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"suika/internal/config"
	"suika/internal/media"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

type Server struct {
	cfg    *config.Config
	router *mux.Router
}

func New(cfg *config.Config) *Server {
	s := &Server{cfg: cfg, router: mux.NewRouter()}
	s.routes()
	return s
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.App.Port)
	log.Printf("suika listening on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) routes() {
	// Static web UI
	s.router.PathPrefix("/js/").Handler(http.StripPrefix("/js/", http.FileServer(http.Dir("web/js"))))

	// Public
	s.router.HandleFunc("/api/v1/login", s.handleLogin).Methods("POST")
	s.router.HandleFunc("/", s.serveIndex).Methods("GET")

	// Protected
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.Use(s.authMiddleware)
	api.HandleFunc("/browse", s.handleBrowse).Methods("GET")
	api.HandleFunc("/images/{imageId}", s.handleImage).Methods("GET")
	api.HandleFunc("/archive-cover/{archiveId}", s.handleArchiveCover).Methods("GET")
	api.HandleFunc("/manga/{archiveId}/pages", s.handleMangaPages).Methods("GET")
	api.HandleFunc("/manga/{archiveId}/page/{page:[0-9]+}", s.handleMangaPage).Methods("GET")
	api.HandleFunc("/manga/{archiveId}/siblings", s.handleMangaSiblings).Methods("GET")
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

// --- Auth ---

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	user := s.cfg.AuthenticateUser(req.Username, req.Password)
	if user == nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user": user.Username,
		"exp":  time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	})

	tokenStr, err := token.SignedString([]byte(s.cfg.App.JWTSecret))
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"token": tokenStr})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ""
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		} else if t := r.URL.Query().Get("token"); t != "" {
			tokenStr = t
		}

		if tokenStr == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(s.cfg.App.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Browse ---

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")

	// Root: list libraries
	if path == "" || path == "." {
		var items []media.BrowseItem
		for _, lib := range s.cfg.Libraries {
			item := media.BrowseItem{
				Name:         filepath.Base(lib.Path),
				FriendlyName: lib.FriendlyName,
				Path:         lib.Path,
				IsDir:        true,
			}
			if cover := findLibCover(lib.Path); cover != "" {
				item.Icon = "/api/v1/images/" + media.EncodePath(cover)
			}
			items = append(items, item)
		}
		writeJSON(w, media.BrowseResponse{Items: items})
		return
	}

	resp, err := media.Browse(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Inject library reading direction
	for _, lib := range s.cfg.Libraries {
		if strings.HasPrefix(path, lib.Path) {
			dir := lib.ReadingDirection
			if dir == "" {
				dir = "rtl"
			}
			if resp.CurrentFolder != nil {
				resp.CurrentFolder.ReadingDirection = dir
			}
			break
		}
	}
	writeJSON(w, resp)
}

func findLibCover(dirPath string) string {
	// Reuse the same cover detection from media package
	entries, _ := filepath.Glob(filepath.Join(dirPath, "cover.*"))
	if len(entries) > 0 {
		return entries[0]
	}
	entries, _ = filepath.Glob(filepath.Join(dirPath, "poster.*"))
	if len(entries) > 0 {
		return entries[0]
	}
	entries, _ = filepath.Glob(filepath.Join(dirPath, "folder.*"))
	if len(entries) > 0 {
		return entries[0]
	}
	return ""
}

// --- Images ---

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	imageId := mux.Vars(r)["imageId"]
	imgPath, err := media.DecodePath(imageId)
	if err != nil {
		http.Error(w, "invalid image id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, imgPath)
}

func (s *Server) handleArchiveCover(w http.ResponseWriter, r *http.Request) {
	archiveId := mux.Vars(r)["archiveId"]
	archivePath, err := media.DecodePath(archiveId)
	if err != nil {
		http.Error(w, "invalid archive id", http.StatusBadRequest)
		return
	}

	data, contentType, err := media.ExtractCover(archivePath)
	if err != nil {
		http.Error(w, "failed to extract cover", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}

// --- Manga Reader ---

func (s *Server) handleMangaPages(w http.ResponseWriter, r *http.Request) {
	archiveId := mux.Vars(r)["archiveId"]
	archivePath, err := media.DecodePath(archiveId)
	if err != nil {
		http.Error(w, "invalid archive id", http.StatusBadRequest)
		return
	}

	pages, err := media.GetPages(archivePath)
	if err != nil {
		http.Error(w, "failed to read archive", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"pages": pages,
		"count": len(pages),
	})
}

func (s *Server) handleMangaPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	archiveId := vars["archiveId"]
	pageStr := vars["page"]

	archivePath, err := media.DecodePath(archiveId)
	if err != nil {
		http.Error(w, "invalid archive id", http.StatusBadRequest)
		return
	}

	pageIndex, err := strconv.Atoi(pageStr)
	if err != nil {
		http.Error(w, "invalid page number", http.StatusBadRequest)
		return
	}

	data, contentType, err := media.GetPageCached(archivePath, pageIndex)
	if err != nil {
		log.Printf("GetPage error: archive=%s page=%d err=%v", archivePath, pageIndex, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Prefetch next pages in background (helps RAR sequential access)
	if pages, err := media.GetPages(archivePath); err == nil {
		media.PrefetchAhead(archivePath, pageIndex, len(pages))
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

func (s *Server) handleMangaSiblings(w http.ResponseWriter, r *http.Request) {
	archiveId := mux.Vars(r)["archiveId"]
	archivePath, err := media.DecodePath(archiveId)
	if err != nil {
		http.Error(w, "invalid archive id", http.StatusBadRequest)
		return
	}

	prev, next := media.GetSiblings(archivePath)

	result := map[string]interface{}{}
	if prev != "" {
		result["prev"] = map[string]string{
			"path": prev,
			"id":   media.EncodePath(prev),
			"name": filepath.Base(prev),
		}
	}
	if next != "" {
		result["next"] = map[string]string{
			"path": next,
			"id":   media.EncodePath(next),
			"name": filepath.Base(next),
		}
	}
	writeJSON(w, result)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
