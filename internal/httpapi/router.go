package httpapi

import (
	"database/sql"
	"encoding/json"
	"github.com/yoshiofthewire/kynotes-server/internal/blobstore"
	"github.com/yoshiofthewire/kynotes-server/internal/config"
	"github.com/yoshiofthewire/kynotes-server/internal/logging"
	"net"
	"net/http"
)

func NewRouter(log *logging.Logger, max int64, ready func() bool, extras ...any) http.Handler {
	mux := http.NewServeMux()
	var db *sql.DB
	var blobs *blobstore.Store
	var cfg config.Config
	for _, extra := range extras {
		switch v := extra.(type) {
		case *sql.DB:
			db = v
		case *blobstore.Store:
			blobs = v
		case config.Config:
			cfg = v
		}
	}
	if db != nil {
		AuthRoutes(mux, db, cfg)
		ContainerRoutes(mux, db)
		SyncRoutes(mux, db)
		CollabRoutes(mux, db)
		PushRoutes(mux, db)
		DeviceRoutes(mux, db, cfg)
		if blobs != nil {
			ObjectRoutes(mux, db, blobs, cfg.Limits.ObjectMaxBytes)
			UploadRoutes(mux, db, blobs, cfg)
		}
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready() {
			WriteError(w, r, 503, "unavailable", "service is not ready")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { WriteError(w, r, 404, "not_found", "not found") })
	var proxies []*net.IPNet
	for _, p := range cfg.Server.TrustedProxies {
		if _, n, e := net.ParseCIDR(p); e == nil {
			proxies = append(proxies, n)
		}
	}
	return SecurityHeaders(MiddlewareWithProxies(log, max, proxies)(AccessLog(log, rateLimitMiddleware(cfg, db, mux))))
}
