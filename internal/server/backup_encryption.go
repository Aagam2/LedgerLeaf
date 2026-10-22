package server

import (
	"billbook/internal/backup"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) encryptedBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	dir, e := s.backupDir()
	if e != nil {
		fail(w, e)
		return
	}
	if r.URL.Path == "/api/backup-encrypt" {
		var p struct {
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if e = decode(w, r, &p); e != nil {
			fail(w, e)
			return
		}
		if !backupName.MatchString(p.Name) {
			fail(w, errors.New("invalid backup name"))
			return
		}
		path := filepath.Join(dir, p.Name)
		if e = backup.Verify(path); e != nil {
			fail(w, e)
			return
		}
		src, e := os.Open(path)
		if e != nil {
			fail(w, e)
			return
		}
		defer src.Close()
		info, e := src.Stat()
		if e != nil {
			fail(w, e)
			return
		}
		temp, e := os.CreateTemp(dir, ".encrypted-*")
		if e != nil {
			fail(w, e)
			return
		}
		defer os.Remove(temp.Name())
		defer temp.Close()
		if e = backup.Encrypt(temp, src, info.Size(), p.Password); e != nil {
			fail(w, e)
			return
		}
		if _, e = temp.Seek(0, 0); e != nil {
			fail(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+strings.TrimSuffix(p.Name, ".db")+`.llbenc"`)
		info, e = temp.Stat()
		if e != nil {
			fail(w, e)
			return
		}
		http.ServeContent(w, r, "backup.llbenc", info.ModTime(), temp)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (513 << 20))
	if e = r.ParseMultipartForm(1 << 20); e != nil {
		fail(w, errors.New("invalid encrypted backup upload"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	src, _, e := r.FormFile("file")
	if e != nil {
		fail(w, e)
		return
	}
	defer src.Close()
	if e = os.MkdirAll(dir, 0700); e != nil {
		fail(w, e)
		return
	}
	temp, e := os.CreateTemp(dir, ".decrypted-*")
	if e != nil {
		fail(w, e)
		return
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if e = backup.Decrypt(temp, src, r.FormValue("password")); e != nil {
		fail(w, e)
		return
	}
	if _, e = temp.Seek(0, 0); e != nil {
		fail(w, e)
		return
	}
	name, e := backup.Import(temp, dir)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 201, map[string]string{"name": name})
}
