package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
  hbmpanel serve [--port 8443]      Jalankan panel HTTP server
  hbmpanel init  [--home PATH]      Inisialisasi data direktori + admin pertama
  hbmpanel version                  Tampilkan versi
`)
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
