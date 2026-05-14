package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"

	"github.com/habibbuchori/hbmpanel/internal/api"
	"github.com/habibbuchori/hbmpanel/internal/config"
	"github.com/habibbuchori/hbmpanel/internal/db"
)

const banner = `
  _    _ ____  __  __ ____                  _
 | |  | |  _ \|  \/  |  _ \ __ _ _ __   ___| |
 | |__| | |_) | \  / | |_) / _| | '_ \ / _ \ |
 |  __  |  _ <| |\/| |  __/ (_| | | | |  __/ |
 |_|  |_|_| \_\_|  |_|_|   \__,_|_| |_|\___|_|
`

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "init":
		cmdInit(os.Args[2:])
	case "reset-password":
		cmdResetPassword(os.Args[2:])
	case "version":
		fmt.Println("hbmpanel", config.Version)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "perintah tidak dikenal: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(banner)
	fmt.Println(`
Usage:
  hbmpanel serve [--port 8443]                 Jalankan panel HTTP server
  hbmpanel init  [--home PATH]                 Inisialisasi data direktori + admin pertama
  hbmpanel reset-password [--user admin]       Reset password (print password baru ke stdout)
  hbmpanel version                             Tampilkan versi
`)
}

func cmdResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	home := fs.String("home", envOr("HBMPANEL_HOME", "/var/lib/hbmpanel"), "direktori data panel")
	user := fs.String("user", "admin", "username")
	_ = fs.Parse(args)

	store, err := db.Open(filepath.Join(*home, "panel.db"))
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	plain, err := generatePassword(18)
	if err != nil {
		log.Fatalf("rand: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}
	n, err := store.UpdatePasswordByName(*user, string(hash))
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	if n == 0 {
		fmt.Fprintf(os.Stderr, "user %q tidak ditemukan\n", *user)
		os.Exit(1)
	}
	fmt.Printf("\n  Password baru untuk %s:\n  %s\n\n", *user, plain)
	fmt.Println("  Simpan & login lewat web panel sekarang juga.")
}

func generatePassword(n int) (string, error) {
	const charset = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8443, "port HTTP")
	home := fs.String("home", envOr("HBMPANEL_HOME", "/var/lib/hbmpanel"), "direktori data panel")
	_ = fs.Parse(args)

	cfg := &config.Config{
		Home: *home,
		Port: *port,
		Log:  envOr("HBMPANEL_LOG", "/var/log/hbmpanel"),
	}

	store, err := db.Open(filepath.Join(cfg.Home, "panel.db"))
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(); err != nil {
		log.Fatalf("migrate db: %v", err)
	}

	app := api.New(cfg, store)
	log.Printf("hbmpanel listening on :%d (home=%s)", cfg.Port, cfg.Home)
	if err := app.Listen(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		log.Fatal(err)
	}
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	home := fs.String("home", "/var/lib/hbmpanel", "direktori data panel")
	_ = fs.Parse(args)

	if err := os.MkdirAll(*home, 0o750); err != nil {
		log.Fatalf("mkdir home: %v", err)
	}

	store, err := db.Open(filepath.Join(*home, "panel.db"))
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(); err != nil {
		log.Fatalf("migrate db: %v", err)
	}

	pass, err := store.EnsureInitialAdmin("admin")
	if err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if pass == "" {
		fmt.Println("admin user sudah ada — skip.")
		return
	}

	credFile := filepath.Join(*home, "initial-credentials.txt")
	content := fmt.Sprintf("username: admin\npassword: %s\n", pass)
	if err := os.WriteFile(credFile, []byte(content), 0o600); err != nil {
		log.Fatalf("write creds: %v", err)
	}
	fmt.Printf("\n  Admin awal dibuat. Kredensial disimpan di:\n  %s\n\n", credFile)
	fmt.Printf("  username: admin\n  password: %s\n\n", pass)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
