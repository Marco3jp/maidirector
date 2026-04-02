package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/Marco3jp/maidirector/internal/api"
	"github.com/Marco3jp/maidirector/internal/config"
	"github.com/Marco3jp/maidirector/internal/context"
	"github.com/Marco3jp/maidirector/internal/llm"
	"github.com/Marco3jp/maidirector/internal/store"
	"github.com/Marco3jp/maidirector/internal/task"
	"github.com/Marco3jp/maidirector/internal/vectorstore"
	"github.com/spf13/cobra"
)

//go:embed all:dist
var embeddedFS embed.FS

var cfgPath string

func main() {
	rootCmd := &cobra.Command{
		Use:   "maidirector",
		Short: "ローカル開発AIアシスタント",
	}

	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "maidirector.yml", "設定ファイルのパス")

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "ダッシュボードサーバーを起動",
		RunE:  runServe,
	}

	rootCmd.AddCommand(serveCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("設定ファイルの読み込みに失敗: %w", err)
	}

	db, err := store.New(cfg.ResolveSQLitePath())
	if err != nil {
		return fmt.Errorf("SQLiteの初期化に失敗: %w", err)
	}
	defer db.Close()

	llmClient := llm.NewOllamaClient(cfg.LLM.Endpoint, cfg.LLM.ChatModel, cfg.LLM.EmbeddingModel)

	vs, err := vectorstore.New(cfg.Qdrant.Endpoint, cfg.Qdrant.Collection, llmClient)
	if err != nil {
		return fmt.Errorf("Qdrantの初期化に失敗: %w", err)
	}

	taskService := task.NewService(db)
	storyService := task.NewStoryService(db)
	ctxManager := context.NewManager(db, vs, llmClient)

	staticFS, err := fs.Sub(embeddedFS, "dist")
	if err != nil {
		return fmt.Errorf("failed to load embedded frontend: %w", err)
	}

	server := api.NewServer(cfg, taskService, storyService, ctxManager, llmClient, vs, staticFS)

	log.Printf("maidirector サーバー起動: http://localhost:%d", cfg.Server.Port)
	return server.ListenAndServe()
}
