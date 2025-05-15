// pkg/cmd/root.go
package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	// Validator
	"github.com/gin-contrib/cors"
	"github.com/go-playground/validator/v10"

	// CLI & Config
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	// DB (PostgreSQL)
	"github.com/jackc/pgx/v5/pgxpool"

	// Profiling
	"github.com/grafana/pyroscope-go"

	// OpenMetrics (Prometheus)
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	// Tracing
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	// Session Manager (Valkey)
	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"

	// HTTP Server
	"github.com/gin-gonic/gin"

	//
	"github.com/aazw/go-base/pkg/api"
	"github.com/aazw/go-base/pkg/api/openapi"
	"github.com/aazw/go-base/pkg/config"
)

const (
	appName                   = "goapp"
	appUsage                  = ""
	envVarPrefix              = "GOAPP"
	defaultConfigFileBasename = "config"
)

var (
	logLevel  string // --log_format
	logFormat string // --log_level
	cfgFile   string // --config
	cfg       config.Config
	rootCmd   = &cobra.Command{
		Use:   appName,
		Short: appUsage,
		Long:  "",
		// Cobra は Execute() 時に PersistentPreRunE → RunE の順で呼ぶ
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig() // 設定ロード & Unmarshal
		},
		RunE: runE,
		// SilenceUsage:  true, // エラー時のusage出力を抑制
		// SilenceErrors: true, // cobraのエラー出力を抑制
	}
)

var handlerOptions = &slog.HandlerOptions{
	AddSource: true, // 行番号などを付与
}
var logger *slog.Logger

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		return fmt.Errorf("rootCmd: %w", err)
	}
	return nil
}

func init() {
	gin.SetMode(gin.ReleaseMode)

	f := rootCmd.PersistentFlags()

	// CLI専用フラグ
	f.StringVar(&logLevel, "log_level", "info", "log level = (info|debug)")
	f.StringVar(&logFormat, "log_format", "text", "log format = (text|json)")
	f.StringVarP(&cfgFile, "config", "c", "", "Config file path")

	// // CLI及びConfig共通フラグ
	// f.String("log_level", "", "log level = (info|debug)")
	// f.String("log_format", "", "log format = (text|json)")

	// Viperへブリッジ
	// viper.BindPFlag("log_level", f.Lookup("log_level"))
	// viper.BindPFlag("log_format", f.Lookup("log_format"))

	// // デフォルト値設定
	// viper.SetDefault("log_level", "info")
	// viper.SetDefault("log_format", "text")

	// 環境変数も取り込む
	viper.SetEnvPrefix(envVarPrefix)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func initConfig() error {
	// log level
	switch strings.ToLower(logLevel) {
	case "info":
		handlerOptions.Level = slog.LevelInfo
	case "debug":
		handlerOptions.Level = slog.LevelDebug
	default:
		return fmt.Errorf("invalid log level found: \"%s\" is invalid", logLevel)
	}

	// log format
	var handler slog.Handler
	switch strings.ToLower(logFormat) {
	case "text":
		handler = slog.NewTextHandler(os.Stderr, handlerOptions)
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, handlerOptions)
	default:
		return fmt.Errorf("invalid log format found: \"%s\" is invalid", logFormat)
	}
	logger = slog.New(handler)

	// config
	cfg = config.NewConfig()

	if cfgFile != "" {
		_, err := os.Stat(cfgFile)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("config file not found: %w", err)
			}
			return fmt.Errorf("internal unknown error: %w", err)
		}
		viper.SetConfigFile(cfgFile)
	} else {
		// use current directory
		cd, _ := os.Getwd()
		viper.AddConfigPath(cd)
		viper.SetConfigName(defaultConfigFileBasename) // config.yaml 等
	}

	// read data
	if err := viper.ReadInConfig(); err != nil {
		if _, notFound := err.(*viper.ConfigFileNotFoundError); notFound {
			logger.Info("config file not found, using default configuration")
		} else {
			return fmt.Errorf("unknown error: %w", err)
		}
	}

	// unmarshal to struct
	if err := viper.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	// validate values
	validate := validator.New(validator.WithRequiredStructEnabled())

	// viperのmapstructureタグ名を優先的に返す関数を登録
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		tag := fld.Tag.Get("mapstructure")
		if tag == "-" || tag == "" {
			return fld.Name // 代替として Go フィールド名
		}
		// `,omitempty`などを除去して最初のトークンだけを返す
		name := strings.Split(tag, ",")[0]
		return name
	})

	// Config
	fmt.Printf("Loaded config: %+v\n", cfg)
	fmt.Println()
	buf, _ := yaml.Marshal(cfg)
	fmt.Printf("%s\n", string(buf))

	// Validation
	err := validate.Struct(cfg)
	if err != nil {
		var invalidValidationError *validator.InvalidValidationError
		if errors.As(err, &invalidValidationError) {
			return fmt.Errorf("validation internal error: %w", err)
		}

		var validateErrs validator.ValidationErrors
		if errors.As(err, &validateErrs) {
			return fmt.Errorf("invalid config: %w", err)
		}

		return fmt.Errorf("internal unknown error: %w", err)
	}

	return nil
}

func runE(cmd *cobra.Command, args []string) (err error) {

	defer func() {
		if err != nil {
			logger.Info("application exited with error")
		} else {
			logger.Info("application exited normally")
		}
	}()

	ctx := context.Background()

	// DB (PostgreSQL)
	dbPool, err := initDB(ctx)
	if err != nil {
		return fmt.Errorf("postgres init error: %w", err)
	}
	defer func() {
		dbPool.Close()
		logger.Info("postgres connection closed normally")
	}()

	// Valkey/Redis
	redisPool, err := initValkey()
	if err != nil {
		return fmt.Errorf("valkey init error: %w", err)
	}
	defer func() {
		if err := redisPool.Close(); err != nil {
			logger.Info("valkey connection closure failed")
			return
		}
		logger.Info("valkey connection closed normally")
	}()

	// Metrics
	err = initMetrics()
	if err != nil {
		return fmt.Errorf("metrics init error: %w", err)
	}

	// Tracing
	shutdown, err := initTracing()
	if err != nil {
		return fmt.Errorf("metrics init error: %w", err)
	}
	defer shutdown()

	tracer := otel.Tracer(appName)

	// Profiling (Pyroscope)
	err = initProfiling()
	if err != nil {
		return fmt.Errorf("profiler init error: %w", err)
	}

	// Session Manager (Valkey)
	sessionManager, err := initSessionManager(redisPool)
	if err != nil {
		return fmt.Errorf("session manager init error: %w", err)
	}

	// Gin
	router, err := setupRouter(sessionManager)
	if err != nil {
		return fmt.Errorf("router init error: %w", err)
	}

	// Add openapi handler
	problemDetailsRenderer, err := api.NewProblemDetailsRenderer("https://example.com/", logger, tracer)
	if err != nil {
		return fmt.Errorf("problem_details_renderer init error: %w", err)
	}
	router.Use(problemDetailsRenderer.Middleware())

	serverImpl := api.NewStrictServerImpl(dbPool, redisPool, sessionManager)
	handler := openapi.NewStrictHandler(serverImpl, nil)
	openapi.RegisterHandlers(router, handler)

	// Run with Graceful Shutdown
	hostport := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(int(cfg.Server.Port)))
	srv := &http.Server{
		Addr:    hostport,
		Handler: router,
	}

	errCh := make(chan error, 1)
	defer close(errCh)
	go func() {
		logger.Info("server listening", "address", hostport)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("listen error: %w", err)
		} else {
			errCh <- nil
		}
		logger.Info("server shutdown")
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case sig := <-quit:
		// 終了シグナルを受け取ったら graceful shutdown
		logger.Info("shutdown signal received", "signal", sig)

		// context.Background() を親に、最大 5 秒後に自動的にキャンセルされる子コンテキスト ctx を生成
		ctx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelTimeout()

		// 新規接続の受付停止、既存接続の完了待ち
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("unknown error: %w", err)
		}
		return nil
	}
}

// PostgreSQL
func initDB(ctx context.Context) (*pgxpool.Pool, error) {

	dsn := &url.URL{
		Scheme: "postgresql",
		Host:   net.JoinHostPort(cfg.Postgres.Host, strconv.Itoa(int(cfg.Postgres.Port))),
		Path:   path.Join("/", cfg.Postgres.Database),
		RawQuery: url.Values{
			"sslmode": []string{cfg.Postgres.SslMode},
		}.Encode(),
		User: url.UserPassword(cfg.Postgres.User, cfg.Postgres.Password),
	}

	pgCfg, err := pgxpool.ParseConfig(dsn.String())
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}
	// https://pkg.go.dev/github.com/jackc/pgx/v4/pgxpool#Config
	pgCfg.MinConns = cfg.Postgres.MinConns
	pgCfg.MaxConns = cfg.Postgres.MaxConns
	pgCfg.MaxConnLifetime = time.Duration(cfg.Postgres.MaxConnLifetimeSeconds) * time.Second
	pgCfg.HealthCheckPeriod = time.Duration(cfg.Postgres.HealthCheckPeriodSeconds) * time.Second

	dbPool, err := pgxpool.NewWithConfig(ctx, pgCfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}

	return dbPool, nil
}

// Valkey/Redis
func initValkey() (*redis.Pool, error) {

	pool := &redis.Pool{
		MaxIdle:     10,
		MaxActive:   100,
		IdleTimeout: 30 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				"tcp",
				// https://pkg.go.dev/github.com/gomodule/redigo/redis#DialOption
				net.JoinHostPort(cfg.Valkey.Host, strconv.Itoa(int(cfg.Valkey.Port))),
				redis.DialConnectTimeout(time.Duration(cfg.Valkey.DialConnectTimeoutSeconds)*time.Second),
				redis.DialReadTimeout(time.Duration(cfg.Valkey.DialReadTimeoutSeconds)*time.Second),
				redis.DialWriteTimeout(time.Duration(cfg.Valkey.DialWriteTimeoutSeconds)*time.Second),
			)
		},
	}

	// 疎通確認(ping)
	conn := pool.Get()
	_, err := redis.String(conn.Do("PING"))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("redis ping error: %w", err)
	}
	conn.Close()

	return pool, nil
}

// Prometheus
var httpRequests *prometheus.HistogramVec

// PrometheusによるPull用エンドポイントのための準備
func initMetrics() error {
	httpRequests = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: appName,
			Subsystem: "http_server",
			Name:      "request_duration_seconds",
			Help:      "HTTP リクエストの処理に要した時間（秒）",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"path", "method", "status"},
	)
	prometheus.MustRegister(httpRequests)

	return nil
}

// Grafana Tempo
func initTracing() (func() error, error) {
	// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp#example-package

	// 内部ロガーをlog/slogに差し替え
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error("opentelemetry export error", "error", err)
	}))

	hostport := net.JoinHostPort(cfg.Tempo.Host, strconv.Itoa(int(cfg.Tempo.Port)))

	ctx := context.Background()
	exp, err := otlptracehttp.New(
		ctx,
		// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.35.0#Option
		otlptracehttp.WithEndpoint(hostport),
		otlptracehttp.WithInsecure(), // TLS なし
	)
	if err != nil {
		return nil, fmt.Errorf("otlptracehttp creation error: %+w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(appName),
		)),
	)
	otel.SetTracerProvider(tracerProvider)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return func() error {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("tracer_provider shutdown error: %+w", err)
		}
		return nil
	}, nil
}

// var pyroscopeLogger = pyroscope.StandardLogger
var pyroscopeLogger = &PyroscopeCustomLogger{}

// Grafana Pyroscope
func initProfiling() error {
	uri := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(cfg.Pyroscope.Host, strconv.Itoa(int(cfg.Pyroscope.Port))),
	}
	hostname, _ := os.Hostname()

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	pyroscope.Start(pyroscope.Config{
		ApplicationName: appName,

		// replace this with the address of pyroscope server
		ServerAddress: uri.String(),

		// you can disable logging by setting this to nil
		Logger: pyroscopeLogger,

		// you can provide static tags via a map:
		Tags: map[string]string{
			"hostname": hostname,
		},

		ProfileTypes: []pyroscope.ProfileType{
			// these profile types are enabled by default:
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,

			// these profile types are optional:
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})

	return nil
}

// Session manager
func initSessionManager(pool *redis.Pool) (*scs.SessionManager, error) {

	sessionManager := scs.New()
	sessionManager.Store = redisstore.New(pool)

	return sessionManager, nil
}

// Gin
func setupRouter(sessionManager *scs.SessionManager) (*gin.Engine, error) {

	// https://github.com/gin-gonic/gin/blob/v1.10.0/gin.go#L224C2-L224C34
	// gin.Default()内では、engine.Use(Logger(), Recovery()) を読んでいる. gin.Logger()が先.
	// router := gin.Default()
	router := gin.New()

	// Custom logger for Access Log
	// https://github.com/gin-gonic/gin/blob/v1.10.0/logger.go#L212-L281
	// https://github.com/gin-gonic/gin/blob/v1.10.0/logger.go#L196-L200
	// https://github.com/gin-gonic/gin/blob/v1.10.0/logger.go#L60
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// ↓デフォルト実装
		// https://github.com/gin-gonic/gin/blob/v1.10.0/logger.go#L141-L161

		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}

		// log/slogで出力する
		var buf bytes.Buffer
		var handler slog.Handler
		switch strings.ToLower(logFormat) {
		case "text":
			handler = slog.NewTextHandler(&buf, handlerOptions)
		case "json":
			handler = slog.NewJSONHandler(&buf, handlerOptions)
		}
		logger := slog.New(handler)

		// https://github.com/gin-gonic/gin/blob/v1.10.0/logger.go#L152-L160
		args := []any{
			// "%v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s"
			"access_timestamp", param.TimeStamp.Format(time.RFC3339Nano),
			"status_code", param.StatusCode,
			"latency", param.Latency,
			"client_ip", param.ClientIP,
			"method", param.Method,
			"path", param.Path,
		}
		if param.ErrorMessage != "" {
			args = append(args, "error_message", param.ErrorMessage)
		}

		logger.Info("gin access log", args...)
		return buf.String()
	}))

	// Default recovery
	router.Use(gin.Recovery())

	// Tracing middleware
	router.Use(otelgin.Middleware(appName))

	// Prometheus middleware
	router.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		status := fmt.Sprint(c.Writer.Status())
		httpRequests.WithLabelValues(c.FullPath(), c.Request.Method, status).Observe(duration)
	})

	// Session Load
	// designed by https://github.com/alexedwards/scs/blob/v2.8.0/session.go#L132
	router.Use(SessionLoadAndSave(sessionManager))

	// Metrics Endpoint: (Prometheus)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// CORS
	if cfg.Server.CORS.Enabled {
		// https://github.com/gin-contrib/cors
		router.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.Server.CORS.AllowOrigins,
			AllowMethods:     cfg.Server.CORS.AllowMethods,
			AllowHeaders:     cfg.Server.CORS.AllowHeaders,
			ExposeHeaders:    cfg.Server.CORS.ExposeHeaders,
			AllowCredentials: cfg.Server.CORS.AllowCredentials,
			MaxAge:           time.Hour * time.Duration(cfg.Server.CORS.MaxAgeHour),
		}))
	}

	return router, nil
}

type sessionWriter struct {
	gin.ResponseWriter
	ctx       context.Context
	req       *http.Request
	sm        *scs.SessionManager
	committed bool
}

func (sw *sessionWriter) commitAndSetCookie() error {
	token, expiry, err := sw.sm.Commit(sw.ctx)
	if err != nil {
		return err
	}
	http.SetCookie(sw.ResponseWriter, &http.Cookie{
		Name:     sw.sm.Cookie.Name,
		Value:    token,
		Path:     sw.sm.Cookie.Path,
		Domain:   sw.sm.Cookie.Domain,
		Secure:   sw.sm.Cookie.Secure,
		HttpOnly: sw.sm.Cookie.HttpOnly,
		SameSite: sw.sm.Cookie.SameSite,
		Expires:  expiry,
	})
	return nil
}

func (sw *sessionWriter) ensureCommit() {
	if sw.committed {
		return
	}
	if err := sw.commitAndSetCookie(); err != nil {
		// ヘッダ未確定なら 500 を返す
		sw.sm.ErrorFunc(sw.ResponseWriter, sw.req, err)
	}
	sw.committed = true
}

func (sw *sessionWriter) WriteHeader(code int) {
	sw.ensureCommit()
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *sessionWriter) Write(data []byte) (int, error) {
	sw.ensureCommit()
	return sw.ResponseWriter.Write(data)
}

func (sw *sessionWriter) WriteString(s string) (int, error) {
	sw.ensureCommit()
	return sw.ResponseWriter.WriteString(s)
}

func SessionLoadAndSave(sm *scs.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {

		// セッション対象外の処理
		switch c.Request.URL.Path {
		case "/metrics":
			c.Next()
			return
		default:
		}

		w := c.Writer
		r := c.Request
		w.Header().Add("Vary", "Cookie")

		// セッションを読込
		var token string
		if cookie, err := r.Cookie(sm.Cookie.Name); err == nil {
			token = cookie.Value
		}
		ctx, err := sm.Load(r.Context(), token)
		if err != nil {
			sm.ErrorFunc(w, r, err)
			c.Abort()
			return
		}
		c.Request = r.WithContext(ctx)

		// ResponseWriter を sessionWriter に差し替え
		cw := &sessionWriter{
			ResponseWriter: w,
			ctx:            ctx,
			req:            r,
			sm:             sm,
		}
		c.Writer = cw

		// ハンドラチェーン続行
		c.Next()

		// ここまで来ても何も書かれていない場合の処理 (204 等)
		if !cw.committed && !cw.Written() {
			cw.ensureCommit()
		}
	}
}

// https://pkg.go.dev/github.com/grafana/pyroscope-go#Logger
// https://github.com/grafana/pyroscope-go/blob/v1.2.2/logger.go#L21
// type Logger interface {
//     Infof(_ string, _ ...interface{})
//     Debugf(_ string, _ ...interface{})
//     Errorf(_ string, _ ...interface{})
// }

type PyroscopeCustomLogger struct{}

func (p *PyroscopeCustomLogger) Infof(format string, args ...any) {

	// https://github.com/grafana/pyroscope-go/blob/v1.2.2/session.go#L80-L85
	switch {
	case format == "starting profiling session:":
		return
	case strings.HasPrefix(format, "  AppName:        "):
		format = "starting profiling session: AppName: %+v"
	case strings.HasPrefix(format, "  Tags:           "):
		format = "starting profiling session: Tags: %+v"
	case strings.HasPrefix(format, "  ProfilingTypes: "):
		format = "starting profiling session: ProfilingTypes: %+v"
	case strings.HasPrefix(format, "  DisableGCRuns:  "):
		format = "starting profiling session: DisableGCRuns: %+v"
	case strings.HasPrefix(format, "  UploadRate:     "):
		format = "starting profiling session: UploadRate: %+v"
	}

	logger.Info("pyroscope", "log", fmt.Sprintf(format, args...))
}

func (p *PyroscopeCustomLogger) Debugf(format string, args ...any) {
	logger.Debug("pyroscope", "log", fmt.Sprintf(format, args...))
}

func (p *PyroscopeCustomLogger) Errorf(format string, args ...any) {
	logger.Error("pyroscope", "log", fmt.Sprintf(format, args...))
}
