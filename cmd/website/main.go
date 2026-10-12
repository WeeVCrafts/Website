package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"

	emailadapter "github.com/wecratfs/commerce/internal/adapters/email"
	firebaseadapter "github.com/wecratfs/commerce/internal/adapters/firebase"
	localpayment "github.com/wecratfs/commerce/internal/adapters/localpayment"
	"github.com/wecratfs/commerce/internal/adapters/meilisearch"
	metricsadapter "github.com/wecratfs/commerce/internal/adapters/metrics"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	razorpayadapter "github.com/wecratfs/commerce/internal/adapters/razorpay"
	redisadapter "github.com/wecratfs/commerce/internal/adapters/redis"
	storageadapter "github.com/wecratfs/commerce/internal/adapters/storage"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	applicationcatalog "github.com/wecratfs/commerce/internal/application/catalog"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	applicationpayment "github.com/wecratfs/commerce/internal/application/payment"
	applicationprivacy "github.com/wecratfs/commerce/internal/application/privacy"
	"github.com/wecratfs/commerce/internal/config"
	"github.com/wecratfs/commerce/internal/ports"
	apiTransport "github.com/wecratfs/commerce/internal/transport/api"
	healthTransport "github.com/wecratfs/commerce/internal/transport/health"
	webTransport "github.com/wecratfs/commerce/internal/transport/web"
	webhookTransport "github.com/wecratfs/commerce/internal/transport/webhook"
	webHandlers "github.com/wecratfs/commerce/internal/web/handlers"
	"github.com/wecratfs/commerce/internal/worker"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg := config.FromEnv()
	if err := validateFirebaseEnvironment(cfg.Environment); err != nil {
		logger.Error("invalid Firebase configuration", "error", err)
		os.Exit(1)
	}
	if err := cfg.ValidateSecurity(); err != nil {
		logger.Error("invalid security configuration", "error", err)
		os.Exit(1)
	}
	if cfg.Environment == "production" && strings.EqualFold(cfg.PaymentMode, "local") && cfg.LocalPaymentSecret == "wecratfs-local-payment" {
		logger.Error("local payment mode in production requires a unique WECRATFS_LOCAL_PAYMENT_SECRET")
		os.Exit(1)
	}

	startupContext, startupCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer startupCancel()
	poolConfig, err := cfg.DatabasePoolConfig()
	if err != nil {
		logger.Error("configure PostgreSQL TLS", "error", err)
		os.Exit(1)
	}
	pool, err := pgxpool.NewWithConfig(startupContext, poolConfig)
	if err != nil {
		logger.Error("create PostgreSQL pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := waitForPostgres(startupContext, pool); err != nil {
		logger.Error("connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	if err := postgres.ApplyMigrations(startupContext, pool); err != nil {
		logger.Error("apply PostgreSQL migrations", "error", err)
		os.Exit(1)
	}
	redisOptions := &redisclient.Options{Addr: cfg.RedisAddress}
	if cfg.RedisURL != "" {
		redisOptions, err = redisclient.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Error("parse Redis URL", "error", err)
			os.Exit(1)
		}
	}
	redisClient := redisclient.NewClient(redisOptions)
	defer redisClient.Close()
	if err := waitForRedis(startupContext, redisClient); err != nil {
		logger.Error("connect to Redis", "error", err)
		os.Exit(1)
	}
	rateLimiter := redisadapter.NewRateLimiter(redisClient)

	catalogService := applicationcatalog.NewService(postgres.NewProductRepository(pool))
	searchRepository := postgres.NewProductSearchRepository(pool)
	searchEngine := meilisearch.NewSearchEngineWithKey(cfg.MeiliAddress, cfg.MeiliAPIKey)
	searchCache := redisadapter.NewCache(redisClient)
	searchMetrics := metricsadapter.NewSearchMetrics()
	searchService := applicationcatalog.NewSearchServiceWithEngineAndObserver(searchRepository, searchCache, searchEngine, searchMetrics)
	passwordHasher := applicationauth.NewArgon2idHasher()
	userRepository := postgres.NewUserRepository(pool)
	authService := applicationauth.NewService(userRepository, passwordHasher)
	sessionService := applicationauth.NewSessionService(redisadapter.NewSessionStore(redisClient)).WithSessionChecker(userRepository)
	var emailSender ports.EmailSender
	if cfg.ResendAPIKey != "" || cfg.ResendAPIKeyFile != "" {
		resendKey := cfg.ResendAPIKey
		if cfg.ResendAPIKeyFile != "" {
			raw, readErr := os.ReadFile(cfg.ResendAPIKeyFile)
			if readErr != nil {
				logger.Error("Resend credential unavailable")
				os.Exit(1)
			}
			resendKey = strings.TrimSpace(string(raw))
		}
		emailSender, err = emailadapter.NewResend(resendKey, cfg.ResendFrom, cfg.ResendReplyTo)
		if err != nil {
			logger.Error("Resend sender configuration invalid")
			os.Exit(1)
		}
		logger.Info("transactional email configured", "provider", "resend")
	} else if cfg.SMTPAddress != "" && cfg.SMTPFrom != "" {
		emailSender = emailadapter.NewSMTP(cfg.SMTPAddress, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	} else {
		logger.Info("account recovery email delivery is disabled; configure WECRATFS_SMTP_ADDR and WECRATFS_SMTP_FROM to enable it")
	}
	recoveryService := applicationauth.NewRecoveryService(postgres.NewAuthActionRepository(pool), passwordHasher, emailSender, cfg.PublicURL)
	commerceRepository := postgres.NewCommerceRepository(pool)
	roleAuthorizer := postgres.NewRoleAuthorizer(pool)
	commerceCatalog := applicationcommerce.NewCatalogServiceWithRoles(commerceRepository, roleAuthorizer)
	inventoryService := applicationcommerce.NewInventoryService(commerceRepository, roleAuthorizer)
	sellerOrderService := applicationcommerce.NewSellerOrderService(commerceRepository, roleAuthorizer)
	sellerReportService := applicationcommerce.NewSellerReportService(commerceRepository, roleAuthorizer)
	sellerAdminService := applicationcommerce.NewSellerAdminService(commerceRepository, roleAuthorizer)
	auditService := applicationcommerce.NewAuditService(commerceRepository, roleAuthorizer)
	roleAdminService := applicationauth.NewRoleAdminService(postgres.NewRoleAdministrationRepository(pool), roleAuthorizer, passwordHasher).WithSessionRevoker(sessionService)
	adminOperationsService := applicationcommerce.NewAdminOperationsService(commerceRepository, roleAuthorizer)
	brandDirectoryService := applicationcommerce.NewBrandDirectoryService(commerceRepository, roleAuthorizer)
	customerDirectoryService := applicationauth.NewCustomerDirectoryService(userRepository, roleAuthorizer)
	financeService := applicationcommerce.NewFinanceService(commerceRepository, roleAuthorizer)
	supportService := applicationcommerce.NewSupportService(commerceRepository, roleAuthorizer)
	returnService := applicationcommerce.NewReturnService(commerceRepository, roleAuthorizer)
	cartService := applicationcommerce.NewCartService(commerceRepository)
	orderService := applicationcommerce.NewOrderService(commerceRepository)
	privacyService := applicationprivacy.NewService(postgres.NewPrivacyRepository(pool))
	paymentRepository := postgres.NewPaymentRepository(pool)
	var paymentGateway ports.PaymentGateway
	var refundGateway ports.PaymentRefundGateway
	var webhookVerifier ports.PaymentWebhookVerifier
	var localGateway *localpayment.Gateway
	if strings.EqualFold(cfg.PaymentMode, "local") {
		localGateway = localpayment.NewGateway(cfg.LocalPaymentSecret)
		paymentGateway, refundGateway, webhookVerifier = localGateway, localGateway, localGateway
		logger.Info("local deterministic payment mode enabled")
	} else {
		gateway := razorpayadapter.NewGateway(cfg.RazorpayKeyID, cfg.RazorpayKeySecret, cfg.RazorpayWebhookSecret, cfg.RazorpayBaseURL)
		paymentGateway, refundGateway, webhookVerifier = gateway, gateway, gateway
	}
	paymentService := applicationpayment.NewService(webhookVerifier, paymentRepository)
	checkoutPaymentService := applicationpayment.NewCheckoutVerificationService(paymentGateway, paymentRepository, paymentRepository)
	refundService := applicationpayment.NewRefundService(refundGateway, paymentRepository)
	mediaStorage, storageErr := storageadapter.NewS3(cfg.S3Address, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3Secure)
	if storageErr != nil {
		logger.Warn("S3 media adapter unavailable", "error", storageErr)
	} else if err := mediaStorage.EnsureBucket(startupContext); err != nil {
		logger.Warn("S3 media bucket unavailable", "error", err)
	}
	mediaService := applicationcommerce.NewMediaService(mediaStorage, postgres.NewMediaRepository(pool), roleAuthorizer)
	supportAttachmentService := applicationcommerce.NewSupportAttachmentService(mediaStorage, commerceRepository, roleAuthorizer)
	paymentOrderService := applicationpayment.NewOrderService(paymentGateway, paymentRepository)
	webHandler, err := webTransport.NewFullHandlerWithPaymentConfigMediaRefundAndRecovery(catalogService, authService, sessionService, commerceCatalog, cartService, orderService, paymentOrderService, searchService, privacyService, mediaService, refundService, recoveryService, cfg.SecureCookies, cfg.RazorpayKeyID)
	if err != nil {
		logger.Error("load web templates", "error", err)
		os.Exit(1)
	}
	webHandler.SetInventoryService(inventoryService)
	webHandler.SetSellerOrderService(sellerOrderService)
	webHandler.SetSellerReportService(sellerReportService)
	webHandler.SetSellerAdminService(sellerAdminService)
	webHandler.SetAuditService(auditService)
	webHandler.SetRoleAdminService(roleAdminService)
	webHandler.SetAdminOperationsService(adminOperationsService)
	webHandler.SetBrandDirectoryService(brandDirectoryService)
	webHandler.SetCustomerDirectoryService(customerDirectoryService)
	webHandler.SetFinanceService(financeService)
	webHandler.SetSupportService(supportService)
	webHandler.SetSupportAttachmentService(supportAttachmentService)
	webHandler.SetReturnService(returnService)
	apiHandler := apiTransport.NewCompleteHandlerWithMediaPaymentRefundAndRecovery(catalogService, searchService, authService, sessionService, cartService, orderService, privacyService, mediaService, paymentOrderService, refundService, recoveryService, cfg.SecureCookies).WithCheckoutPaymentVerification(checkoutPaymentService).WithNoPaymentDemo(cfg.NoPaymentDemo)
	var firebaseProvider *firebaseadapter.Provider
	if project := os.Getenv("WECRATFS_FIREBASE_PROJECT_ID"); project != "" {
		credentialFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if credentialJSON := os.Getenv("WECRATFS_FIREBASE_CREDENTIALS_JSON"); credentialJSON != "" {
			file, writeErr := os.CreateTemp("", "weevcrafts-firebase-*.json")
			if writeErr != nil {
				logger.Error("Firebase credential staging unavailable", "error", writeErr)
				os.Exit(1)
			}
			if _, writeErr = file.WriteString(credentialJSON); writeErr != nil {
				_ = file.Close()
				logger.Error("Firebase credential staging unavailable", "error", writeErr)
				os.Exit(1)
			}
			_ = file.Close()
			credentialFile = file.Name()
			defer os.Remove(credentialFile)
		}
		firebaseProvider, err = firebaseadapter.New(startupContext, project, credentialFile)
		if err != nil {
			logger.Error("Firebase configuration unavailable", "error", err)
			os.Exit(1)
		}
		firebaseRepository := postgres.NewFirebaseRepository(pool)
		firebaseService := applicationauth.NewFirebaseService(firebaseProvider, firebaseRepository, authService, userRepository)
		recoveryService.WithFirebaseEmails(firebaseProvider, firebaseRepository, cfg.PublicURL+"/")
		apiHandler.WithFirebase(firebaseService)
		sessionService.WithSessionChecker(firebaseService)
		sessionService.WithRevocationCleanup(func(ctx context.Context, userID int64, sessionID string) error {
			if sessionID != "" {
				_, err := pool.Exec(ctx, `DELETE FROM customer_push_subscriptions WHERE session_hash=$1`, pushHash(sessionID))
				return err
			}
			_, err := pool.Exec(ctx, `DELETE FROM customer_push_subscriptions WHERE user_id=$1`, userID)
			return err
		})
	}
	backgroundWorker, err := worker.New(1, 16)
	if err != nil {
		logger.Error("configure background worker", "error", err)
		os.Exit(1)
	}
	workerContext, stopWorker := context.WithCancel(context.Background())
	backgroundWorker.Start(workerContext)
	productIndexProcessor := applicationcatalog.NewProductIndexProcessorWithCache(postgres.NewOutboxRepository(pool), searchRepository, searchEngine, searchCache)
	sellerDispatchProcessor := applicationcommerce.NewSellerDispatchProcessor(postgres.NewOutboxRepository(pool), commerceRepository, emailSender, cfg.PublicURL)
	orderConfirmationProcessor := applicationcommerce.NewOrderConfirmationProcessor(postgres.NewOutboxRepository(pool), commerceRepository, emailSender, cfg.PublicURL)
	paidOrderConfirmationProcessor := applicationcommerce.NewPaidOrderConfirmationProcessor(postgres.NewOutboxRepository(pool), commerceRepository, emailSender, cfg.PublicURL, cfg.PaymentMode == "local")
	go worker.RunPeriodic(workerContext, 10*time.Second, func(ctx context.Context) error {
		processContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := orderConfirmationProcessor.Process(processContext, 20)
		_, paidErr := paidOrderConfirmationProcessor.Process(processContext, 20)
		if err == nil {
			err = paidErr
		}
		if err != nil {
			logger.Warn("order confirmations failed", "error", err)
		}
		return err
	})
	go worker.RunPeriodic(workerContext, time.Minute, func(ctx context.Context) error {
		cleanupContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, indexErr := productIndexProcessor.Process(cleanupContext, 10)
		if indexErr != nil {
			logger.Warn("product index processing failed", "error", indexErr)
		}
		return indexErr
	})
	go worker.RunPeriodic(workerContext, time.Minute, func(ctx context.Context) error {
		processContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, dispatchErr := sellerDispatchProcessor.Process(processContext, 20)
		if dispatchErr != nil {
			logger.Warn("seller dispatch notifications failed", "error", dispatchErr)
		}
		return dispatchErr
	})
	go worker.RunPeriodic(workerContext, time.Minute, func(ctx context.Context) error {
		cleanupContext, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, cleanupErr := commerceRepository.ReleaseExpiredReservations(cleanupContext, 100)
		if cleanupErr != nil {
			logger.Warn("release expired reservations failed", "error", cleanupErr)
		}
		return cleanupErr
	})
	go worker.RunPeriodic(workerContext, time.Minute, func(ctx context.Context) error {
		cleanupContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, cleanupErr := mediaService.CleanupExpired(cleanupContext, 100)
		if cleanupErr != nil && !errors.Is(cleanupErr, applicationcommerce.ErrInvalidMedia) {
			logger.Warn("expired media cleanup failed", "error", cleanupErr)
		}
		return cleanupErr
	})
	defer stopWorker()
	healthHandler := healthTransport.NewHandlerWithMetrics(map[string]healthTransport.Probe{
		"postgresql": func(ctx context.Context) error { return postgres.Ping(ctx, pool) },
		"redis":      func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
	}, func() any { return searchMetrics.Snapshot() })

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", healthHandler.Live)
	mux.HandleFunc("/health/ready", healthHandler.Ready)
	mux.HandleFunc("/health/metrics", healthHandler.Metrics)
	robotsHandler, sitemapHandler := seoHandler(pool, cfg.PublicURL)
	mux.HandleFunc("/robots.txt", robotsHandler)
	mux.HandleFunc("/sitemap.xml", sitemapHandler)
	mux.HandleFunc("/api/v1/products", apiHandler.Products)
	mux.HandleFunc("/api/v1/products/", apiHandler.Products)
	mux.HandleFunc("/api/v1/search", apiHandler.Search)
	mux.HandleFunc("/api/v1/auth/register", apiHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", apiHandler.Login)
	mux.HandleFunc("/api/v1/auth/firebase/session", apiHandler.FirebaseSession)
	mux.HandleFunc("/api/v1/auth/firebase/verification", apiHandler.FirebaseVerificationRequest)
	mux.HandleFunc("/api/v1/firebase/config", firebaseConfiguration(firebaseProvider != nil))
	mux.HandleFunc("/firebase-messaging-sw.js", firebaseWorker)
	if firebaseProvider != nil {
		push := &customerPush{pool, redisClient, sessionService, firebaseProvider}
		mux.HandleFunc("/api/v1/customer/push", push.Handle)
		mux.HandleFunc("/api/v1/customer/push/test", push.Handle)
		go worker.RunPeriodic(workerContext, time.Minute, func(ctx context.Context) error {
			err := runFirebaseProcessing(ctx, push)
			if err != nil {
				logger.Warn("Firebase background processing unavailable")
			}
			return err
		})
	}
	mux.HandleFunc("/api/v1/auth/session", apiHandler.CurrentSession)
	mux.HandleFunc("/api/v1/auth/logout", apiHandler.Logout)
	mux.HandleFunc("/api/v1/auth/password-reset/request", apiHandler.PasswordResetRequest)
	mux.HandleFunc("/api/v1/auth/password-reset/confirm", apiHandler.PasswordResetConfirm)
	mux.HandleFunc("/api/v1/auth/email-verification/request", apiHandler.EmailVerificationRequest)
	mux.HandleFunc("/api/v1/auth/email-verification/confirm", apiHandler.EmailVerificationConfirm)
	mux.HandleFunc("/api/v1/cart", apiHandler.Cart)
	mux.HandleFunc("/api/v1/cart/add", apiHandler.CartAdd)
	mux.HandleFunc("/api/v1/wishlist", apiHandler.Wishlist)
	mux.HandleFunc("/api/v1/wishlist/add", apiHandler.WishlistAdd)
	mux.HandleFunc("/api/v1/wishlist/", apiHandler.WishlistRemove)
	mux.HandleFunc("/api/v1/checkout", apiHandler.Checkout)
	mux.HandleFunc("/api/v1/checkout/quote", apiHandler.CheckoutQuote)
	mux.HandleFunc("/api/v1/checkout/no-payment", apiHandler.NoPaymentCheckout)
	mux.HandleFunc("/api/v1/orders", apiHandler.Orders)
	mux.HandleFunc("/api/v1/orders/", apiHandler.Order)
	mux.HandleFunc("/api/v1/privacy", apiHandler.PrivacyCenter)
	mux.HandleFunc("/api/v1/privacy/consent", apiHandler.PrivacyConsent)
	mux.HandleFunc("/api/v1/privacy/requests", apiHandler.PrivacyRequest)
	mux.HandleFunc("/api/v1/privacy/delete", apiHandler.PrivacyDelete)
	mux.HandleFunc("/api/v1/seller/products/media-url", apiHandler.MediaUploadURL)
	mux.HandleFunc("/api/v1/seller/products/media-upload/", apiHandler.MediaUpload)
	mux.HandleFunc("/api/v1/seller/products/media-finalize", apiHandler.MediaFinalize)
	mux.HandleFunc("/api/v1/seller/products/media-delete", apiHandler.MediaDelete)
	if cfg.LocalMode && !strings.EqualFold(cfg.Environment, "production") {
		mux.Handle("/api/v1/local/", apiTransport.NewLocalDemoHandler(pool, sessionService))
	}
	mux.Handle("/webhooks/razorpay", webhookTransport.NewRazorpayHandler(paymentService))
	if developmentPaymentHelpersEnabled(cfg) {
		mux.HandleFunc("/dev/payments/", localPaymentHandler(pool, sessionService, paymentService, localGateway, paymentRepository))
	}
	// The configured demo provider has a capture-only checkout endpoint. Keep
	// development failure/pending controls disabled in production.
	registerDemoPaymentCapture(mux, cfg, localPaymentHandler(pool, sessionService, paymentService, localGateway, paymentRepository))
	portalHandler := webHandlers.NewLivePortalHandler(pool, sessionService, webHandler)
	portalHandler.EnableLocalDeliverySimulation(cfg.LocalMode)
	portalHandler.SetRuntimeProbes(cfg.PaymentMode, map[string]func(context.Context) error{
		"database": pool.Ping,
		"redis":    func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
		"search":   searchHealthProbe(cfg.MeiliAddress),
		"storage":  mediaStorage.CheckHealth,
	})
	portalHandler.SetRoleAdminService(roleAdminService)
	portalHandler.SetSupportAttachmentService(supportAttachmentService)
	storefrontHandler := webHandlers.NewLiveStorefrontHandler(catalogService, searchService, cartService, orderService, sessionService, pool, cfg.PaymentMode, cfg.RazorpayKeyID, cfg.RazorpayKeyID != "" && cfg.RazorpayKeySecret != "", webHandler, cfg.NoPaymentDemo, cfg.PublicURL, supportAttachmentService)
	storefrontHandler.SetProfileStorage(mediaStorage)
	go worker.RunPeriodic(workerContext, time.Minute, func(ctx context.Context) error {
		cleanupContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return storefrontHandler.CleanupAvatars(cleanupContext)
	})
	mux.HandleFunc("/access-unavailable", storefrontHandler.AccessUnavailable)
	// Keep customer support mutations and protected attachment reads ahead of
	// the broad /support/ portal route. The portal still owns support-agent GET
	// pages and delegates its mutation endpoints to webHandler.
	mux.Handle("/support/create", webHandler)
	mux.Handle("/support/customer-reply", webHandler)
	mux.Handle("/support/reply", webHandler)
	mux.Handle("/support/status", webHandler)
	mux.Handle("/support/attachments/", webHandler)
	mux.Handle("/notifications", portalHandler)
	mux.Handle("/notifications/read", portalHandler)
	mux.Handle("/ui/notifications", portalHandler)
	mux.Handle("/ui/admin/orders/status", portalHandler)
	mux.Handle("/workspaces", portalHandler)
	mux.Handle("/admin", portalHandler)
	mux.Handle("/admin/", portalHandler)
	mux.Handle("/finance", portalHandler)
	mux.Handle("/finance/", portalHandler)
	mux.Handle("/marketplace-admin", portalHandler)
	mux.Handle("/marketplace-admin/", portalHandler)
	mux.Handle("/super-admin", portalHandler)
	mux.Handle("/super-admin/", portalHandler)
	mux.Handle("/finance-portal", portalHandler)
	mux.Handle("/finance-portal/", portalHandler)
	mux.Handle("/operations-portal", portalHandler)
	mux.Handle("/operations-portal/", portalHandler)
	mux.Handle("/seller-admin", portalHandler)
	mux.Handle("/seller-admin/", portalHandler)
	mux.Handle("/seller", portalHandler)
	mux.Handle("/seller/", portalHandler)
	mux.Handle("/support-portal", portalHandler)
	mux.Handle("/support-portal/", portalHandler)
	mux.Handle("/support", portalHandler)
	mux.Handle("/support/", portalHandler)
	mux.Handle("/", storefrontHandler)

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           requestID(authRateLimit(accountSecurity(canonicalHostRedirect(mux, cfg.PublicURL, cfg.Environment), sessionService), rateLimiter)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	serverError := make(chan error, 1)
	go func() {
		logger.Info("Weevcrafts server listening", "address", cfg.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	shutdownSignal, stopSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()
	select {
	case <-shutdownSignal.Done():
		stopWorker()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := backgroundWorker.Shutdown(shutdownContext); err != nil {
			logger.Error("background worker shutdown failed", "error", err)
		}
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	case err := <-serverError:
		logger.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}

func developmentPaymentHelpersEnabled(cfg config.Config) bool {
	return cfg.Environment != "production" && cfg.LocalMode && strings.EqualFold(cfg.PaymentMode, "local")
}

func registerDemoPaymentCapture(mux *http.ServeMux, cfg config.Config, capture http.HandlerFunc) {
	if !strings.EqualFold(cfg.PaymentMode, "local") {
		return
	}
	mux.HandleFunc("/api/v1/payments/demo-capture/", func(w http.ResponseWriter, r *http.Request) {
		number := strings.TrimPrefix(r.URL.Path, "/api/v1/payments/demo-capture/")
		if number == "" || strings.ContainsAny(number, "/\\") || number != strings.TrimSpace(number) || len(number) > 120 {
			writeLocalJSON(w, http.StatusNotFound, map[string]string{"error": "payment_order_not_found"})
			return
		}
		// Reuse the authenticated, CSRF-protected, owner-scoped payment lifecycle.
		request := r.Clone(r.Context())
		request.URL.Path = "/dev/payments/" + number + "/capture"
		capture(w, request)
	})
}

func localPaymentHandler(pool *pgxpool.Pool, sessions *applicationauth.SessionService, service *applicationpayment.Service, gateway *localpayment.Gateway, paymentRepo ports.PaymentWebhookRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeLocalJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		cookie, err := r.Cookie("wecratfs_session")
		if err != nil || sessions == nil {
			writeLocalJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
			return
		}
		session, err := sessions.Get(r.Context(), cookie.Value)
		if err != nil || session.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			writeLocalJSON(w, http.StatusForbidden, map[string]string{"error": "csrf_validation_failed"})
			return
		}
		parts := strings.Split(strings.TrimPrefix(strings.Trim(r.URL.Path, "/"), "dev/payments/"), "/")
		if len(parts) != 2 || parts[0] == "" || (parts[1] != "capture" && parts[1] != "fail" && parts[1] != "pending") {
			writeLocalJSON(w, http.StatusNotFound, map[string]string{"error": "unknown_local_payment_action"})
			return
		}
		orderNumber := parts[0]
		var orderID int64
		var providerOrderID, currency string
		var amount int64
		if err := pool.QueryRow(r.Context(), `
			SELECT o.id, COALESCE(pa.provider_order_id, ''), COALESCE(pa.amount_cents, o.total_cents), COALESCE(pa.currency, o.currency, 'INR')
			FROM orders o LEFT JOIN payment_attempts pa ON pa.order_id = o.id
			WHERE o.order_number = $1 AND o.user_id = $2
			ORDER BY pa.id DESC NULLS LAST LIMIT 1`, orderNumber, session.UserID).Scan(&orderID, &providerOrderID, &amount, &currency); err != nil {
			writeLocalJSON(w, http.StatusNotFound, map[string]string{"error": "payment_order_not_found"})
			return
		}
		if providerOrderID == "" {
			providerOrderID = "local_order_" + orderNumber
			_, _ = pool.Exec(r.Context(), `
				INSERT INTO payment_attempts (order_id, provider, provider_order_id, status, amount_cents, currency)
				VALUES ($1, 'local', $2, 'pending', $3, $4)
				ON CONFLICT DO NOTHING`, orderID, providerOrderID, amount, currency)
			_, _ = pool.Exec(r.Context(), `
				UPDATE payment_attempts
				SET provider_order_id = $1, provider = 'local', status = 'pending', updated_at = NOW()
				WHERE order_id = $2 AND (provider_order_id IS NULL OR provider_order_id = '')`, providerOrderID, orderID)
		}
		eventType, status := "payment.pending", "pending"
		if parts[1] == "capture" {
			eventType, status = "payment.captured", "captured"
		} else if parts[1] == "fail" {
			eventType, status = "payment.failed", "failed"
		}
		eventID := "local_evt_" + orderNumber + "_" + parts[1]
		paymentID := localpayment.PaymentID(orderNumber)
		body, err := localpayment.WebhookPayload(eventID, eventType, providerOrderID, paymentID, status, amount, currency)
		if err != nil {
			writeLocalJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		var processed bool
		if service != nil && gateway != nil {
			processed, err = service.HandleWebhook(r.Context(), body, gateway.Sign(string(body)))
		}
		if err != nil || (!processed && paymentRepo != nil) {
			webhook := ports.PaymentWebhook{
				EventID:           eventID,
				EventType:         eventType,
				ProviderOrderID:   providerOrderID,
				ProviderPaymentID: paymentID,
				Status:            status,
				AmountCents:       amount,
				Currency:          currency,
			}
			processed, err = paymentRepo.ProcessWebhook(r.Context(), webhook, body)
		}
		if err != nil {
			writeLocalJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeLocalJSON(w, http.StatusOK, map[string]any{"order_number": orderNumber, "action": parts[1], "processed": processed})
	}
}

func writeLocalJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func authRateLimit(next http.Handler, limiter ports.RateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAuthenticationAttempt(r.URL.Path) && !(r.URL.Path == "/login" && r.Method == http.MethodGet) {
			key := "auth:" + clientAddress(r) + ":" + authenticationAttemptBucket(r.URL.Path)
			allowed, err := limiter.Allow(r.Context(), key, 20, time.Minute)
			if err != nil {
				writeRateLimitAPIError(w, r, http.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "Authentication is temporarily unavailable.")
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", "60")
				writeRateLimitAPIError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many authentication attempts.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isAuthenticationAttempt(path string) bool {
	switch path {
	case "/api/v1/customer/push/test", "/api/v1/customer/push", "/api/v1/auth/firebase/session", "/api/v1/auth/firebase/verification", "/login", "/register", "/forgot-password", "/reset-password", "/verify-email", "/account/verify-email",
		"/api/v1/auth/register", "/api/v1/auth/login",
		"/api/v1/auth/password-reset/request", "/api/v1/auth/password-reset/confirm",
		"/api/v1/auth/email-verification/request", "/api/v1/auth/email-verification/confirm":
		return true
	default:
		return false
	}
}

func authenticationAttemptBucket(path string) string {
	switch path {
	case "/api/v1/auth/firebase/session", "/login", "/api/v1/auth/login":
		return "login"
	case "/register", "/api/v1/auth/register":
		return "register"
	case "/forgot-password", "/reset-password", "/api/v1/auth/password-reset/request", "/api/v1/auth/password-reset/confirm":
		return "password-recovery"
	case "/verify-email", "/account/verify-email", "/api/v1/auth/email-verification/request", "/api/v1/auth/email-verification/confirm", "/api/v1/auth/firebase/verification":
		return "email-verification"
	default:
		return "other"
	}
}

func clientAddress(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func writeRateLimitAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		http.Error(w, message, status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
		"code": code, "message": message, "request_id": w.Header().Get("X-Request-ID"),
	}})
}

func waitForPostgres(ctx context.Context, pool *pgxpool.Pool) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastErr error
	for {
		if lastErr = postgres.Ping(ctx, pool); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return lastErr
		case <-ticker.C:
		}
	}
}

func waitForRedis(ctx context.Context, client *redisclient.Client) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastErr error
	for {
		if lastErr = client.Ping(ctx).Err(); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return lastErr
		case <-ticker.C:
		}
	}
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = time.Now().UTC().Format("20060102T150405.000000000Z07:00")
		}
		w.Header().Set("X-Request-ID", requestID)
		r = r.WithContext(ports.WithRequestID(r.Context(), requestID))
		next.ServeHTTP(w, r)
	})
}
