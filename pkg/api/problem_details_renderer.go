package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"

	"goapp/pkg/api/openapi"
)

func init() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		panic("gin validator init internal error")
	}
	// これで、 validator.ValidationErrors の  fe.Namespace() や fe.Field() で構造体のフィールド名の代わりに返すものを決められる (カスタマイズできる)
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

var tracer = otel.Tracer("ginapp")

type ProblemDetailsRenderer struct {
	uriReference string
	logger       *slog.Logger
	tracer       oteltrace.Tracer
}

func NewProblemDetailsRenderer(uriReferenceBase string, logger *slog.Logger, tracer oteltrace.Tracer) (*ProblemDetailsRenderer, error) {

	// uriReferenceBase
	uriRef, err := url.Parse(uriReferenceBase)
	if err != nil {
		return nil, fmt.Errorf("problem_details_renderer init error: %w", err)
	}
	uriRef.Path = path.Join("/", "/validation-error")
	uriReference := uriRef.String()

	// logger
	if logger == nil {
		logger = slog.Default()
	}

	return &ProblemDetailsRenderer{
		uriReference: uriReference,
		logger:       logger,
		tracer:       tracer,
	}, nil
}

func (p *ProblemDetailsRenderer) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		_, span := tracer.Start(c.Request.Context(), "problem_details_renderer")
		defer span.End()
		traceID := span.SpanContext().TraceID().String()

		c.Next()

		if len(c.Errors) == 0 {
			// 異常終了していない
			return
		}

		ge := c.Errors.Last() // *gin.Error
		if ge == nil {
			// エラーが無ければ何もしない
			return
		}

		var verrs validator.ValidationErrors
		if errors.As(ge.Err, &verrs) {
			// ここで JSON をまだ書いていなければ上書きする
			if !c.Writer.Written() {
				invalidParams := make([]openapi.InvalidParam, 0, len(verrs))

				for _, fe := range verrs {
					// JSONフィールド名
					fieldName := fe.Namespace()[strings.IndexByte(fe.Namespace(), '.')+1:]

					// 独自関数でメッセージ生成
					verb := p.validationMsg(fe)
					errMsg := fmt.Sprintf("'%s' %s", fieldName, verb)

					invalidParams = append(invalidParams, openapi.InvalidParam{
						Name:   fieldName,
						Reason: errMsg,
					})
				}

				status := c.Writer.Status()
				if status < 400 {
					status = http.StatusBadRequest
				}

				c.AbortWithStatusJSON(status, openapi.ProblemDetails{
					Type:          stringPointer(p.uriReference),
					Title:         stringPointer(http.StatusText(status)),
					Status:        intPointer(status),
					Detail:        stringPointer("validation failed for one or more fields"),
					InvalidParams: &invalidParams,
					TraceId:       &traceID,
				})
				return
			}
		}
	}
}

// Readableなメッセージ生成
func (p *ProblemDetailsRenderer) validationMsg(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	default:
		p.logger.Error("unsupported validation pattern error", "error", fe)

		return "is invalid"
	}
}
