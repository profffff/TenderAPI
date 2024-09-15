package main

import (
	"github.com/joho/godotenv"
	"log"
	"log/slog"
	"my_zad/api"
	"os"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {

	//TODO: init config: cleanenv
	//cfg := config.MustLoad()
	if err := godotenv.Load(); err != nil {
		log.Fatalf("err load env vars")
	}

	//TODO: init logger: slog
	//	log := setupLogger(cfg.Env)
	//	log.Info("starting", slog.String("env", cfg.Env))
	//	log.Debug("Debug enabled")

	//TODO: init storage:
	store, err := api.NewPostgresStorage()

	if err != nil {
		log.Fatal(err)
	}

	if err := store.Init(); err != nil {
		log.Fatal(err)
	}

	//TODO: run server
	server := api.NewAPIServer(os.Getenv("SERVER_ADDRESS"), store) //8082
	server.Run()

}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
