package web

import (
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// backupDB entrega uma cópia completa do banco (finan.db): registros,
// configurações da IA, conversa com o consultor e aparelhos pareados.
// O backup portátil (.json) é gerado no próprio aparelho, na tela Exportar.
func (s *Server) backupDB(w http.ResponseWriter, r *http.Request) {
	dir, err := os.MkdirTemp("", "finan-backup-")
	if err != nil {
		serverError(w, err)
		return
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "finan.db")
	if err := s.store.BackupTo(r.Context(), path); err != nil {
		serverError(w, err)
		return
	}
	name := "finan_rich_" + time.Now().Format("2006-01-02") + ".db"
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}
