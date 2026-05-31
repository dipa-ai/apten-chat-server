package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/config"
	"github.com/apten-chat/messenger/internal/db"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/files"
	"github.com/apten-chat/messenger/internal/invite"
	"github.com/apten-chat/messenger/internal/message"
	"github.com/apten-chat/messenger/internal/push"
	"github.com/apten-chat/messenger/internal/user"
	"github.com/apten-chat/messenger/internal/ws"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Run migrations.
	log.Println("running migrations...")
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	queries := dbq.New(pool)

	// Bootstrap: create a seed invite if no users exist.
	if count, err := queries.CountUsers(ctx); err != nil {
		log.Fatalf("count users: %v", err)
	} else if count == 0 {
		inv, err := invite.Bootstrap(ctx, queries, cfg.InviteTTL)
		if err != nil {
			log.Fatalf("bootstrap invite: %v", err)
		}
		log.Printf("no users found — bootstrap invite code: %s", inv)
	}

	// WebSocket hub.
	hub := ws.NewHub()
	hub.OnDisconnect = func(userID int64) {
		queries.UpdateLastSeen(context.Background(), userID)
	}
	go hub.Run()

	// Cross-replica realtime bridge (optional). When REDIS_ADDR is set, events
	// are published to Redis Pub/Sub and events from other replicas are fanned
	// out to this instance's local clients via SendLocal (never Send, which
	// would re-publish and loop).
	if cfg.RedisAddr != "" {
		brokerCtx, cancelBroker := context.WithCancel(ctx)
		defer cancelBroker()
		rdb := redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		defer rdb.Close()
		broker := ws.NewBroker(rdb, cfg.InstanceID, hub.SendLocal, hub.BroadcastAllLocal)
		hub.Broker = broker
		go broker.Run(brokerCtx)
		log.Printf("ws: redis bridge enabled (instance %q)", cfg.InstanceID)
	}

	// Services.
	userService := user.NewService(pool, queries, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	inviteService := invite.NewService(queries, cfg.InviteTTL)
	chatService := chat.NewService(pool, queries)
	messageService := message.NewService(queries)

	s3Client, err := files.NewS3Client(ctx, cfg.S3Bucket, cfg.S3Region, cfg.S3Endpoint)
	if err != nil {
		log.Fatalf("s3: %v", err)
	}
	fileService := files.NewService(s3Client, queries, cfg.FileMaxSize)
	pushService := push.NewService(queries, hub, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDContact)

	// Handlers.
	userHandler := user.NewHandler(userService, queries)
	inviteHandler := invite.NewHandler(inviteService)
	chatHandler := chat.NewHandler(chatService, queries)
	messageHandler := message.NewHandler(messageService, chatService, queries, hub)
	fileHandler := files.NewHandler(fileService, chatService, hub)
	pushHandler := push.NewHandler(pushService, queries)
	wsHandler := ws.NewHandler(hub, chatService, messageService, queries, cfg.JWTSecret, cfg.WSAllowedOrigins)

	// Router.
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// WebSocket (token in query param, no middleware).
	r.Get("/api/ws", wsHandler.ServeHTTP)

	// Public routes with rate limiting.
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(5, time.Minute))
		r.Post("/api/login", userHandler.Login)
		r.Post("/api/refresh", userHandler.Refresh)
	})
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(3, time.Hour))
		r.Post("/api/register", userHandler.Register)
	})

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg.JWTSecret))

		r.Get("/api/users/me", userHandler.GetMe)
		r.Put("/api/users/me", userHandler.UpdateMe)
		r.Get("/api/users", userHandler.ListUsers)

		// Chats.
		r.Get("/api/chats", chatHandler.List)
		r.Post("/api/chats", chatHandler.Create)
		r.Get("/api/chats/{id}", chatHandler.Get)
		r.Put("/api/chats/{id}", chatHandler.Update)
		r.Post("/api/chats/{id}/members", chatHandler.AddMember)
		r.Delete("/api/chats/{id}/members/{uid}", chatHandler.RemoveMember)

		// Messages.
		r.Get("/api/chats/{id}/messages", messageHandler.ListMessages)
		r.Get("/api/chats/{id}/messages/{mid}", messageHandler.GetMessage)
		r.Put("/api/chats/{id}/messages/{mid}", messageHandler.EditMessage)
		r.Delete("/api/chats/{id}/messages/{mid}", messageHandler.DeleteMessage)

		// Files.
		r.Post("/api/chats/{id}/upload", fileHandler.Upload)
		r.Get("/api/files/{fileID}", fileHandler.Download)
		r.Get("/api/files/{fileID}/thumb", fileHandler.Thumbnail)

		// Push notifications.
		r.Post("/api/push/subscribe", pushHandler.Subscribe)
		r.Delete("/api/push/subscribe", pushHandler.Unsubscribe)
		r.Get("/api/push/vapid-key", pushHandler.VAPIDKey)

		// Admin-only routes.
		r.Group(func(r chi.Router) {
			r.Use(auth.AdminOnly)
			r.Post("/api/invites", inviteHandler.Create)
			r.Get("/api/invites", inviteHandler.List)
			r.Delete("/api/invites/{id}", inviteHandler.Delete)
		})
	})

	// Server with graceful shutdown.
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("server stopped")
}
