package config

// Version diset via ldflags saat release build:
//   -ldflags="-X github.com/habibbuchori/hbmpanel/internal/config.Version=v1.2.3"
var Version = "0.1.0-dev"

type Config struct {
	Home string
	Port int
	Log  string
}
