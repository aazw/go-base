package api

import (
	"log/slog"
	"net/http"
	"net/url"
	"path"

	"github.com/gin-gonic/gin"

	"github.com/aazw/go-base/pkg/api/openapi"
	"github.com/aazw/go-base/pkg/cerrors"
)

type RequestSizeLimiter struct {
	uriReference string
	logger       *slog.Logger
}

func NewRequestSizeLimiter(uriReferenceBase string, logger *slog.Logger) (*RequestSizeLimiter, error) {

	// uriReferenceBase
	uriRef, err := url.Parse(uriReferenceBase)
	if err != nil {
		return nil, cerrors.ErrSystemInternal.New(
			cerrors.WithCause(err),
			cerrors.WithMessage("failed to initialize problem details renderer"),
			cerrors.WithMessagef("url: %s", uriReferenceBase),
		)
	}
	uriRef.Path = path.Join("/", "/validation-error")
	uriReference := uriRef.String()

	// logger
	if logger == nil {
		logger = slog.Default()
	}

	return &RequestSizeLimiter{
		uriReference: uriReference,
		logger:       logger,
	}, nil
}

func (p *RequestSizeLimiter) Middleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Content-Length で事前チェック
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, openapi.ProblemDetails{
				Type:   PtrOrNil(p.uriReference),
				Title:  PtrOrNil(http.StatusText(http.StatusRequestEntityTooLarge)),
				Status: PtrOrNil(int32(413)),
				Detail: PtrOrNil("Your request body is too large."),
			})
			return
		}

		// MaxBytesReader で読み込み中の過剰もカバー
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		// ハンドラー実行
		c.Next()

		// Gin が 413 にしてくれた場合にもカスタムJSON
		if c.Writer.Status() == http.StatusRequestEntityTooLarge {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, openapi.ProblemDetails{
				Type:   PtrOrNil(p.uriReference),
				Title:  PtrOrNil(http.StatusText(http.StatusRequestEntityTooLarge)),
				Status: PtrOrNil(int32(413)),
				Detail: PtrOrNil("Your request body is too large."),
			})
			return
		}
	}
}
