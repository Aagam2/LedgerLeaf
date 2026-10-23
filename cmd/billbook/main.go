package main

import (
	"billbook/internal/database"
	"billbook/internal/instance"
	"billbook/internal/logging"
	"billbook/internal/recovery"
	"billbook/internal/server"
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Set by release builds; development builds remain identifiable.
var version = "dev"
var revision = "unknown"
var builtAt = "unknown"

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "Ledgerleaf:", e)
		os.Exit(1)
	}
}
func run() error {
	data := flag.String("data-dir", "", "Optional portable data directory")
	noBrowser := flag.Bool("no-browser", false, "Do not open a browser automatically")
	showVersion := flag.Bool("version", false, "Show version and build information without opening data")
	recoverAdmin := flag.String("recover-admin", "", "Reset an existing administrator password offline; requires --data-dir and a stopped application")
	restoreBackup := flag.String("restore-backup", "", "Offline restore from a verified plaintext backup; requires --data-dir and --confirm-restore")
	confirmRestore := flag.Bool("confirm-restore", false, "Confirm replacement of all businesses and users from the backup")
	restoreEncrypted := flag.Bool("restore-encrypted", false, "Decrypt --restore-backup using a password read from standard input (one line)")
	databasePasswordStdin := flag.Bool("database-password-stdin", false, "Unlock/create an encrypted working database with one password line on standard input (SQLCipher build)")
	encryptDatabase := flag.Bool("encrypt-database", false, "Convert an existing stopped plaintext installation to SQLCipher; requires --data-dir and --database-password-stdin; preserves plaintext original")
	flag.Parse()
	if *showVersion {
		fmt.Printf("Ledgerleaf %s (revision %s; built %s; %s/%s)\n", version, revision, builtAt, runtime.GOOS, runtime.GOARCH)
		return nil
	}
	if *encryptDatabase && (!*databasePasswordStdin || *data == "" || *restoreBackup != "" || *recoverAdmin != "") {
		return fmt.Errorf("encryption conversion requires explicit --data-dir and --database-password-stdin, without recovery/restore")
	}
	if *databasePasswordStdin && *restoreEncrypted {
		return fmt.Errorf("restore the encrypted backup first, then use --encrypt-database to encrypt the working database")
	}
	var databasePassword string
	if *databasePasswordStdin {
		line, e := bufio.NewReaderSize(os.Stdin, 512).ReadSlice('\n')
		if e != nil {
			return fmt.Errorf("provide a database password as one newline-terminated line on standard input")
		}
		databasePassword = strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
		for i := range line {
			line[i] = 0
		}
		if !database.ValidDatabasePassword(databasePassword) {
			return fmt.Errorf("database password must be 12–256 bytes without line breaks or NUL")
		}
		if !database.CipherAvailable() {
			return fmt.Errorf("use a SQLCipher-enabled build for database encryption")
		}
	}

	if *restoreEncrypted && *restoreBackup == "" {
		return fmt.Errorf("--restore-encrypted requires --restore-backup")
	}
	if *confirmRestore && *restoreBackup == "" {
		return fmt.Errorf("--confirm-restore requires --restore-backup")
	}
	if *restoreBackup != "" && (*data == "" || !*confirmRestore || *recoverAdmin != "") {
		return fmt.Errorf("offline restore requires explicit --data-dir and --confirm-restore, without --recover-admin")
	}
	if *recoverAdmin != "" {
		if *data == "" {
			return fmt.Errorf("administrator recovery requires an explicit --data-dir")
		}
		if _, e := os.Stat(filepath.Join(*data, "data", "billbook.db")); e != nil {
			return fmt.Errorf("recovery requires an existing installation: %w", e)
		}
	}
	if *data == "" {
		d, e := os.UserConfigDir()
		if e != nil {
			return e
		}
		*data = filepath.Join(d, "BillBook")
	}
	abs, e := filepath.Abs(*data)
	if e != nil {
		return e
	}
	*data = abs
	for _, d := range []string{"data", "backups", "attachments", "exports", "logs", "settings"} {
		if e = os.MkdirAll(filepath.Join(*data, d), 0700); e != nil {
			return e
		}
	}
	lock, e := instance.Lock(filepath.Join(*data, "settings", "instance.lock"))
	if e != nil {
		return fmt.Errorf("another instance may be using this data directory: %w", e)
	}
	defer lock.Close()
	lf, e := logging.Open(filepath.Join(*data, "logs", "application.log"), 5<<20, 3)
	if e != nil {
		return e
	}
	defer lf.Close()
	logger := log.New(lf, "", log.LstdFlags)
	logger.Println("starting")
	if *encryptDatabase {
		preserved, e := recovery.EncryptOffline(*data, databasePassword)
		if e != nil {
			return e
		}
		fmt.Println("Working database encrypted. Plaintext original retained in:", preserved)
		fmt.Println("Start with --database-password-stdin. Existing backups remain plaintext; protect them separately.")
		return nil
	}
	if *restoreBackup != "" {
		if databasePassword != "" {
			return fmt.Errorf("offline restore produces plaintext; restore first, then run --encrypt-database")
		}
		var preserved string
		if *restoreEncrypted {
			// Never accept passwords in command-line arguments or log them.
			reader := bufio.NewReaderSize(os.Stdin, 512)
			line, readErr := reader.ReadSlice('\n')
			if readErr != nil {
				return fmt.Errorf("provide the backup password as one newline-terminated line on standard input")
			}
			password := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
			preserved, e = recovery.RestoreEncryptedOffline(*data, *restoreBackup, password)
			for i := range line {
				line[i] = 0
			}
		} else {
			preserved, e = recovery.RestoreOffline(*data, *restoreBackup)
		}
		if e != nil {
			return e
		}
		fmt.Println("Verified backup restored. Original database/WAL/SHM files retained in:", preserved)
		fmt.Println("Start Ledgerleaf and sign in with the credentials from that backup.")
		return nil
	}
	db, e := database.OpenEncrypted(filepath.Join(*data, "data", "billbook.db"), databasePassword)
	if e != nil {
		return fmt.Errorf("cannot open the database: %w. Preserve the complete data directory. To restore a known-good snapshot while stopped, run with --data-dir <installation> --restore-backup <snapshot.db> --confirm-restore; original files will be retained", e)
	}
	defer db.Close()
	if *recoverAdmin != "" {
		password, snapshot, e := recovery.ResetAdmin(db, filepath.Join(*data, "backups"), *recoverAdmin)
		if e != nil {
			return e
		}
		fmt.Println("Administrator password reset. Recovery backup:", snapshot)
		fmt.Println("Temporary password:", password)
		fmt.Println("Start Ledgerleaf and change this password in Users & permissions.")
		logger.Println("administrator password recovered locally")
		return nil
	}
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return e
	}
	defer listener.Close()
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	app, e := server.New(db, listener.Addr().String(), stop)
	if e != nil {
		return e
	}
	app.DataDir = *data
	if databasePassword != "" {
		app.OpenDatabase = func(path string) (*sql.DB, error) { return database.OpenEncrypted(path, databasePassword) }
		app.PrepareRestore = func(path string) error { return recovery.EncryptRestoreStage(path, databasePassword) }
		fmt.Println("Working database encryption is active. Local backup snapshots remain plaintext; encrypted downloads are available.")
	}

	defer func() { app.DB.Close() }()
	backupContext, stopBackups := context.WithCancel(context.Background())
	backupDone := make(chan struct{})
	go func() {
		defer close(backupDone)
		app.RunBackupSchedule(backupContext, func(err error) { logger.Println("scheduled backup:", err) })
	}()
	defer func() { stopBackups(); <-backupDone }()
	srv := &http.Server{Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384, ErrorLog: logger}
	// Install before publishing readiness: a supervisor may terminate the app
	// immediately after seeing its launch URL.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, shutdownSignals()...)
	defer signal.Stop(signals)
	errs := make(chan error, 1)
	go func() { errs <- srv.Serve(listener) }()
	url := "http://" + listener.Addr().String() + "/#launch=" + app.Token
	fmt.Println("Ledgerleaf is running. Use Exit application in the browser or Ctrl+C to stop.")
	fmt.Println(url)
	if !*noBrowser {
		if e = openBrowser(url); e != nil {
			logger.Println("browser launch failed:", e)
			fmt.Println("Open the launch link above in your browser.")
		}
	}
	select {
	case <-done:
	case <-signals:
	case e = <-errs:
		if e != http.ErrServerClosed {
			return e
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	e = srv.Shutdown(ctx)
	if e != nil {
		srv.Close()
	}
	logger.Println("stopped")
	return e
}
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
