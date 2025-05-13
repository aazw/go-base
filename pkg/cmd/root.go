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

	// HTTP Server
	"github.com/gin-gonic/gin"

	// Validator
	"github.com/go-playground/validator/v10"

	// CLI & Config
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	// DB (PostgreSQL)
	"github.com/jackc/pgx/v5/pgxpool"

	// Session Manager (Valkey)
	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"

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

	//
	"goapp/pkg/config"
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

func runE(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Config
	fmt.Printf("Loaded config: %+v\n", cfg)
	fmt.Println()
	buf, _ := yaml.Marshal(cfg)
	fmt.Printf("%s\n", string(buf))

	// DB (PostgreSQL)
	dbPool, err := initDB(ctx)
	if err != nil {
		return fmt.Errorf("postgres init error: %w", err)
	}
	defer dbPool.Close()

	// Session Manager (Valkey)
	sessionManager, err := initSessionManager()
	if err != nil {
		return fmt.Errorf("session manager init error: %w", err)
	}

	// Profiling (Pyroscope)
	err = initProfiling()
	if err != nil {
		return fmt.Errorf("profiler init error: %w", err)
	}

	// Gin
	router, err := setupRouter()
	if err != nil {
		return fmt.Errorf("router init error: %w", err)
	}

	// add routes
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	// Run with Graceful Shutdown
	hostport := net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port)))
	srv := &http.Server{
		Addr:    hostport,
		Handler: sessionManager.LoadAndSave(router),
	}

	errCh := make(chan error, 1)
	defer close(errCh)
	go func() {
		fmt.Printf("server listening on %s\n", "0.0.0.0:8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("listen error: %w", err)
		} else {
			errCh <- nil
		}
		fmt.Printf("server shutdown\n")
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case sig := <-quit:
		// 終了シグナルを受け取ったら graceful shutdown
		fmt.Printf("shutdown signal received: %v\n", sig)

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

	cfg, err := pgxpool.ParseConfig(dsn.String())
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}
	cfg.MinConns = 2
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour
	cfg.HealthCheckPeriod = time.Minute

	dbPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}

	return dbPool, nil
}

func initSessionManager() (*scs.SessionManager, error) {
	pool := &redis.Pool{
		MaxIdle:     10,
		MaxActive:   100,
		IdleTimeout: 30 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				"tcp",
				net.JoinHostPort(cfg.Valkey.Host, strconv.Itoa(int(cfg.Valkey.Port))),
				redis.DialConnectTimeout(3*time.Second),
				redis.DialReadTimeout(3*time.Second),
				redis.DialWriteTimeout(3*time.Second),
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

	// session manager
	sessionManager := scs.New()
	sessionManager.Store = redisstore.New(pool)

	return sessionManager, nil
}

func initTracing() (func() error, error) {
	// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp#example-package

	// // 内部ロガーをlog/slogに差し替え
	// otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
	// 	sharedLogger.Error("opentelemetry export error", "error", err)
	// }))

	hostport := net.JoinHostPort(cfg.Tempo.Host, strconv.Itoa(int(cfg.Tempo.Port)))

	ctx := context.Background()
	exp, err := otlptracehttp.New(
		ctx,
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
		// Logger: pyroscopeLogger,

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

func setupRouter() (*gin.Engine, error) {
	// Gin
	gin.SetMode(gin.ReleaseMode)

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
		logger.Info(
			"gin access log",
			// "%v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s"
			"access_timestamp", fmt.Sprintf("%v", param.TimeStamp.Format("2006/01/02 - 15:04:05")),
			"status_code", fmt.Sprintf("%3d", param.StatusCode),
			"latency", fmt.Sprintf("%13v", param.Latency),
			"client_ip", fmt.Sprintf("%15s", param.ClientIP),
			"method", fmt.Sprintf("%-7s", param.Method),
			"path", fmt.Sprintf("%#v", param.Path),
			"error_message", param.ErrorMessage,
		)
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

	// Metrics Endpoint: (Prometheus)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return router, nil
}
