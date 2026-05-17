package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wujunqi/rc_wujunqi/internal/config"
	"github.com/wujunqi/rc_wujunqi/internal/controller"
	"github.com/wujunqi/rc_wujunqi/internal/domain"
	"github.com/wujunqi/rc_wujunqi/internal/infra/httpclient"
	mysqlinfra "github.com/wujunqi/rc_wujunqi/internal/infra/mysql"
	"github.com/wujunqi/rc_wujunqi/internal/infra/rabbitmq"
	"github.com/wujunqi/rc_wujunqi/internal/repository"
	"github.com/wujunqi/rc_wujunqi/internal/service"
	"github.com/wujunqi/rc_wujunqi/internal/worker"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db := mustOpenMySQL(ctx, cfg.MySQLDSN, logger)
	defer db.Close()
	if err := mysqlinfra.ApplyMigrations(ctx, db); err != nil {
		logger.Fatalf("apply migrations: %v", err)
	}

	mq := mustOpenRabbitMQ(ctx, cfg.RabbitMQURL, logger)
	defer mq.Close()
	if err := mq.SetupTopology(); err != nil {
		logger.Fatalf("setup rabbitmq topology: %v", err)
	}

	repo := repository.NewTaskRepository(db)
	vendorProfiles, err := config.LoadVendorProfiles(cfg.VendorProfilesPath)
	if err != nil {
		logger.Fatalf("load vendor profiles: %v", err)
	}
	notificationService := service.NewNotificationServiceWithConfig(repo, vendorProfiles, config.DefaultAppCredentials())
	router := gin.Default()
	authMiddleware := controller.NewAuthMiddleware(config.DefaultAppCredentials(), controller.NewMemoryNonceStore(), 5*time.Minute)
	controller.NewNotificationController(notificationService, authMiddleware).RegisterRoutes(router)

	if cfg.RunWorkers {
		notifier := httpclient.NewNotifier(cfg.HTTPTimeout)
		outboxPublisher := worker.NewOutboxPublisher(repo, mq, cfg.OutboxInterval, 20, logger)
		deliveryConsumer := worker.NewDeliveryConsumer(repo, mq, notifier, domain.DefaultRetryPolicy(), logger)
		go outboxPublisher.Run(ctx)
		go deliveryConsumer.Run(ctx, mq, "notification-delivery-consumer", cfg.ConsumerPrefetch)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Printf("server listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("server shutdown failed: %v", err)
	}
}

func mustOpenMySQL(ctx context.Context, dsn string, logger *log.Logger) *sql.DB {
	var lastErr error
	for i := 0; i < 30; i++ {
		db, err := mysqlinfra.Open(ctx, dsn)
		if err == nil {
			return db
		}
		lastErr = err
		logger.Printf("waiting for mysql: %v", err)
		time.Sleep(2 * time.Second)
	}
	logger.Fatalf("open mysql: %v", lastErr)
	return nil
}

func mustOpenRabbitMQ(ctx context.Context, url string, logger *log.Logger) *rabbitmq.Client {
	var lastErr error
	for i := 0; i < 30; i++ {
		client, err := rabbitmq.Dial(url)
		if err == nil {
			return client
		}
		lastErr = err
		logger.Printf("waiting for rabbitmq: %v", err)
		select {
		case <-ctx.Done():
			logger.Fatalf("open rabbitmq canceled: %v", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
	logger.Fatalf("open rabbitmq: %v", lastErr)
	return nil
}
