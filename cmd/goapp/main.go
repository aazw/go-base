// cmd/goapp/main.go
package main

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
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	// HTTP Server
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	// Validator
	"github.com/go-playground/validator/v10"

	// CLI & Config
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	// DB (PostgreSQL)
	"github.com/jackc/pgx/v5/pgxpool"

	// Profiling (Pyroscope)
	"github.com/grafana/pyroscope-go"

	// OpenMetrics (Prometheus)
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	// OpenTelemetry
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	// Session Manager (Valkey)
	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"

	//
	"github.com/aazw/go-base/pkg/api"
	"github.com/aazw/go-base/pkg/api/openapi"
	"github.com/aazw/go-base/pkg/cerrors"
	"github.com/aazw/go-base/pkg/config"
	"github.com/aazw/go-base/pkg/db/postgres"
	"github.com/aazw/go-base/pkg/operations"
)

var (
	GoVersion  = "unknown"
	MainModule = "unknown"
)

const (
	appName                   = "goapp"
	appUsage                  = ""
	envVarPrefix              = "GOAPP"
	defaultConfigFileBasename = "config"
)

const (
	logLevelFlagKey  string = "log_level"
	logFormatFlagKey string = "log_format"
	configFlagKey    string = "config"
)

var (
	cfg     config.Config
	rootCmd = &cobra.Command{
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

var tracer = otel.Tracer(appName)

func init() {
	// ビルド情報取得
	if info, ok := debug.ReadBuildInfo(); ok {
		GoVersion = info.GoVersion  // 例: go1.21.0
		MainModule = info.GoVersion // 例: github.com/aazw/go-base (devel)
	}

	// gin
	gin.SetMode(gin.ReleaseMode)

	// viper and cobra
	f := rootCmd.PersistentFlags()

	// CLIフラグ
	f.String(logLevelFlagKey, "info", "log level = (info|debug)")
	f.String(logFormatFlagKey, "text", "log format = (text|json)")
	f.StringP(configFlagKey, "c", "", "Config file path")

	// 環境変数も取り込む
	viper.SetEnvPrefix(envVarPrefix)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 明示的な BindEnv
	viper.BindEnv(configFlagKey, envVarPrefix+"_CONFIG")        // GOAPP_CONFIG → config
	viper.BindEnv(logLevelFlagKey, envVarPrefix+"_LOG_LEVEL")   // GOAPP_LOG_LEVEL → log_level
	viper.BindEnv(logFormatFlagKey, envVarPrefix+"_LOG_FORMAT") // GOAPP_LOG_FORMAT → log_format

	// フラグと viper のバインド
	viper.BindPFlag(logLevelFlagKey, f.Lookup(logLevelFlagKey))
	viper.BindPFlag(logFormatFlagKey, f.Lookup(logFormatFlagKey))
	viper.BindPFlag(configFlagKey, f.Lookup(configFlagKey))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
	logger.Info("application exited normally")
}

func initConfig() error {
	// log level
	logLevel := viper.GetString(logLevelFlagKey)
	switch strings.ToLower(logLevel) {
	case "info":
		handlerOptions.Level = slog.LevelInfo
	case "debug":
		handlerOptions.Level = slog.LevelDebug
	default:
		return cerrors.ErrValidation.New(
			cerrors.WithMessagef("invalid log level: %s", logLevel),
		)
	}

	// log format
	logFormat := viper.GetString(logFormatFlagKey)
	var handler slog.Handler
	switch strings.ToLower(logFormat) {
	case "text":
		handler = slog.NewTextHandler(os.Stderr, handlerOptions)
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, handlerOptions)
	default:
		return cerrors.ErrValidation.New(
			cerrors.WithMessagef("invalid log format: %s", logFormat),
		)
	}
	logger = slog.New(handler)

	// config
	cfg = config.NewConfig()
	cfgFile := viper.GetString(configFlagKey)
	if cfgFile != "" {
		_, err := os.Stat(cfgFile)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return cerrors.ErrValidation.New(
					cerrors.WithCause(err),
					cerrors.WithMessage("config file not found"),
				)
			}
			return cerrors.ErrSystemInternal.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("failed to check config file"),
			)
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
			return cerrors.ErrSystemInternal.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("failed to read config file"),
			)
		}
	}

	// unmarshal to struct
	if err := viper.Unmarshal(&cfg); err != nil {
		return cerrors.ErrValidation.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to unmarshal config"),
		)
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
			return cerrors.ErrSystemInternal.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("validation internal error"),
			)
		}

		var validateErrs validator.ValidationErrors
		if errors.As(err, &validateErrs) {
			return cerrors.ErrValidation.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("invalid config"),
			)
		}

		return cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("unknown validation error"),
		)
	}

	return nil
}

func runE(cmd *cobra.Command, args []string) (err error) {

	ctx := context.Background()

	// DB (PostgreSQL)
	dbPool, err := newPostgresPool(ctx)
	if err != nil {
		return cerrors.AppendCheckpoint(
			err,
			cerrors.WithCheckpointMessage("failed to initialize postgres connection"),
		)
	}
	defer func() {
		dbPool.Close()
		logger.Info("postgres connection closed normally")
	}()

	dbHandler, err := postgres.NewHandler(dbPool)
	if err != nil {
		return cerrors.AppendCheckpoint(
			err,
			cerrors.WithCheckpointMessage("failed to initialize database handler"),
		)
	}

	opsHander, err := operations.NewHandler(dbHandler)
	if err != nil {
		return cerrors.AppendCheckpoint(
			err,
			cerrors.WithCheckpointMessage("failed to initialize API handler"),
		)
	}

	// Valkey/Redis
	redisPool, err := newValkeyPool(ctx)
	if err != nil {
		return cerrors.AppendCheckpoint(
			err,
			cerrors.WithCheckpointMessage("failed to initialize redis connection"),
		)
	}
	defer func() {
		if err := redisPool.Close(); err != nil {
			logger.Info("valkey connection closure failed")
			return
		}
		logger.Info("valkey connection closed normally")
	}()

	// OpenTelemetry
	if cfg.OTLPTrace.Enabled || cfg.OTLPMetric.Enabled || cfg.OTLPLog.Enabled {
		otelShutdown, err := setupOTelSDK(ctx)
		if err != nil {
			return cerrors.AppendCheckpoint(
				err,
				cerrors.WithCheckpointMessage("failed to initialize OpenTelemetry SDK"),
			)
		}
		defer func() {
			err = errors.Join(err, otelShutdown(context.Background()))
		}()
	}

	// Prometheus
	if cfg.Prometheus.Enabled {
		err = newPrometheus(ctx)
		if err != nil {
			return cerrors.AppendCheckpoint(
				err,
				cerrors.WithCheckpointMessage("failed to initialize metrics"),
			)
		}
	}

	// Profiling (Pyroscope)
	if cfg.Pyroscope.Enabled {
		err = newProfiler()
		if err != nil {
			return cerrors.AppendCheckpoint(
				err,
				cerrors.WithCheckpointMessage("failed to initialize profiler"),
			)
		}
	}

	// Session Manager (Valkey)
	sessionManager, err := initSessionManager(redisPool)
	if err != nil {
		return cerrors.AppendCheckpoint(
			err,
			cerrors.WithCheckpointMessage("failed to initialize session manager"),
		)
	}

	// Gin
	router, err := setupRouter(sessionManager)
	if err != nil {
		return cerrors.AppendCheckpoint(
			err,
			cerrors.WithCheckpointMessage("failed to initialize router"),
		)
	}

	// Add openapi handler
	problemDetailsRenderer, err := api.NewProblemDetailsRenderer("https://example.com/", logger, tracer)
	if err != nil {
		return cerrors.AppendCheckpoint(
			err,
			cerrors.WithCheckpointMessage("failed to initialize problem details renderer"),
		)
	}
	router.Use(problemDetailsRenderer.Middleware())

	serverImpl := api.NewStrictServerImpl(opsHander, dbPool, redisPool, sessionManager)
	handler := openapi.NewStrictHandler(serverImpl, nil)
	openapi.RegisterHandlers(router, handler)

	// Run with Graceful Shutdown
	hostport := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(int(cfg.Server.Port)))
	srv := &http.Server{
		Addr:              hostport,
		Handler:           router,
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,       // リクエストヘッダ＋ボディ読み込み完了までの最大時間
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,      // レスポンス書き込みまでの最大時間
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,       // Keep-Alive 接続の最大アイドル時間
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeoutSeconds) * time.Second, // ヘッダ読み込みのタイムアウト
	}

	errCh := make(chan error, 1)
	defer close(errCh)
	go func() {
		logger.Info("server listening", "address", hostport)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- cerrors.ErrUnavailable.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("failed to start server"),
			)
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
			return cerrors.ErrUnavailable.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("failed to shutdown server"),
			)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return cerrors.AppendCheckpoint(
				err,
				cerrors.WithCheckpointMessage("unexpected error occurred"),
			)
		}
		return nil
	}
}

// PostgreSQL
func newPostgresPool(ctx context.Context) (*pgxpool.Pool, error) {

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
		return nil, cerrors.ErrValidation.New(
			cerrors.WithCause(err),
			cerrors.WithMessagef("dns=%s", dsn.String()),
		)
	}
	// https://pkg.go.dev/github.com/jackc/pgx/v4/pgxpool#Config
	pgCfg.MinConns = cfg.Postgres.MinConns
	pgCfg.MaxConns = cfg.Postgres.MaxConns
	pgCfg.MaxConnLifetime = time.Duration(cfg.Postgres.MaxConnLifetimeSeconds) * time.Second
	pgCfg.HealthCheckPeriod = time.Duration(cfg.Postgres.HealthCheckPeriodSeconds) * time.Second

	dbPool, err := pgxpool.NewWithConfig(ctx, pgCfg)
	if err != nil {
		return nil, cerrors.ErrDBConnection.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to create database connection pool"),
		)
	}

	return dbPool, nil
}

// Valkey/Redis
func newValkeyPool(_ context.Context) (*redis.Pool, error) {

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
		return nil, cerrors.ErrDBConnection.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to connect to redis"),
		)
	}
	conn.Close()

	return pool, nil
}

// https://opentelemetry.io/docs/languages/go/getting-started/#initialize-the-opentelemetry-sdk
func setupOTelSDK(ctx context.Context) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil

		if err != nil {
			return cerrors.ErrUnavailable.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("failed to shutdown server"),
			)
		}
		return nil
	}

	// 内部ロガーをlog/slogに差し替え
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error("opentelemetry export error", "error", err)
	}))

	// Set up propagator
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider
	if cfg.OTLPTrace.Enabled {
		tracerProvider, err := newOTLPTracerProvider(ctx)
		if err != nil {
			return shutdown, errors.Join(err, shutdown(ctx))
		}
		shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
		otel.SetTracerProvider(tracerProvider)
	}

	// Set up meter provider
	if cfg.OTLPMetric.Enabled {
		meterProvider, err := newOTLPMeterProvider(ctx)
		if err != nil {
			return shutdown, errors.Join(err, shutdown(ctx))
		}
		shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
		otel.SetMeterProvider(meterProvider)
	}

	// Set up logger provider
	if cfg.OTLPLog.Enabled {
		loggerProvider, err := newOTLPLoggerProvider(ctx)
		if err != nil {
			return shutdown, errors.Join(err, shutdown(ctx))
		}
		shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
		global.SetLoggerProvider(loggerProvider)
	}

	return shutdown, nil
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// For Grafana Tempo
func newOTLPTracerProvider(ctx context.Context) (*trace.TracerProvider, error) {
	// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp#example-package

	hostport := net.JoinHostPort(cfg.OTLPTrace.Host, strconv.Itoa(int(cfg.OTLPTrace.Port)))

	// exporter
	traceExporter, err := otlptracehttp.New(
		ctx,
		// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.35.0#Option
		otlptracehttp.WithEndpoint(hostport),
		otlptracehttp.WithInsecure(), // TLS なし
	)
	if err != nil {
		return nil, cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to create trace exporter"),
		)
	}

	// provider
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(
			traceExporter,
			// Default is 5s. Set to 1s for demonstrative purposes.
			trace.WithBatchTimeout(time.Second),
		),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(appName),
		)),
	)
	return tracerProvider, nil
}

// PrometheusによるPull用エンドポイントのための準備
func newOTLPMeterProvider(ctx context.Context) (*metric.MeterProvider, error) {

	hostport := net.JoinHostPort(cfg.OTLPMetric.Host, strconv.Itoa(int(cfg.OTLPMetric.Port)))

	// exporter
	metricExporter, err := otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithEndpoint(hostport),
		otlpmetrichttp.WithInsecure(), // TLS なし
	)
	if err != nil {
		return nil, cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to create metric exporter"),
		)
	}

	// provider
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			// Default is 1m. Set to 3s for demonstrative purposes.
			metric.WithInterval(3*time.Second))),
	)
	return meterProvider, nil
}

func newOTLPLoggerProvider(ctx context.Context) (*log.LoggerProvider, error) {

	hostport := net.JoinHostPort(cfg.OTLPLog.Host, strconv.Itoa(int(cfg.OTLPLog.Port)))

	// exporter
	logExporter, err := otlploghttp.New(
		ctx,
		otlploghttp.WithEndpoint(hostport),
		otlploghttp.WithInsecure(), // TLS なし
	)
	if err != nil {
		return nil, cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to create log exporter"),
		)
	}

	// provider
	loggerProvider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	)
	return loggerProvider, nil
}

// Prometheus
var httpRequests *prometheus.HistogramVec

func newPrometheus(_ context.Context) error {
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

// var pyroscopeLogger = pyroscope.StandardLogger
var pyroscopeLogger = &PyroscopeCustomLogger{}

// Grafana Pyroscope
func newProfiler() error {
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

	// Custom gin logger for Access Log
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
		logFormat := viper.GetString(logFormatFlagKey)
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

	// rate limiter
	if cfg.Server.RateLimit.Enabled {
		router.Use(api.RateLimiter(1, 5))
	}

	// max request tize
	if cfg.Server.MaxRequestSize <= 0 {
		sizeLimiter, err := api.NewRequestSizeLimiter("https://example.com/", logger)
		if err != nil {
			return nil, cerrors.ErrSystemInternal.New(
				cerrors.WithCause(err),
				cerrors.WithMessage("failed to init request size limiter"),
			)
		}
		router.Use(sizeLimiter.Middleware(cfg.Server.MaxRequestSize))
	}

	// Tracing middleware
	if cfg.OTLPTrace.Enabled || cfg.OTLPMetric.Enabled || cfg.OTLPLog.Enabled {
		router.Use(otelgin.Middleware(appName))
	}

	// Prometheus middleware
	if cfg.Prometheus.Enabled {
		// Custom Metrics
		router.Use(func(c *gin.Context) {
			start := time.Now()
			c.Next()
			duration := time.Since(start).Seconds()
			status := fmt.Sprint(c.Writer.Status())
			httpRequests.WithLabelValues(c.FullPath(), c.Request.Method, status).Observe(duration)
		})

		// Metrics Endpoint: (Prometheus)
		router.GET(cfg.Prometheus.MetricsPath, gin.WrapH(promhttp.Handler()))
	}

	// add Custom Headers
	if len(cfg.Server.CustomHeaders) > 0 {
		router.Use(func(c *gin.Context) {
			for _, customHeader := range cfg.Server.CustomHeaders {
				if customHeader.Enabled && customHeader.Name != "" && customHeader.Value != "" {
					responseHeader := c.Writer.Header()
					if _, ok := responseHeader[customHeader.Name]; ok {
						if customHeader.Override {
							c.Header(customHeader.Name, customHeader.Value)
						}
					} else {
						c.Header(customHeader.Name, customHeader.Value)
					}
				}
			}
			c.Next()
		})
	}

	// Session Load
	// designed by https://github.com/alexedwards/scs/blob/v2.8.0/session.go#L132
	router.Use(SessionLoadAndSave(sessionManager))

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
