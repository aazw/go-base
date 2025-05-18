// pkg/api/problem_details_renderer.go
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

	"github.com/aazw/go-base/pkg/api/openapi"
	"github.com/aazw/go-base/pkg/cerrors"
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

	verb, ok := p.verbForTag(fe.Tag())
	if !ok && p.logger != nil {
		p.logger.Error("verb_for_tag error", "error", fe)
	}
	return verb
}

func (p *ProblemDetailsRenderer) verbForTag(tag string) (string, bool) {
	// https://github.com/go-playground/validator
	// https://github.com/go-playground/validator/blob/v10.26.0/baked_in.go
	switch tag {
	// bakedInAliases
	// https://github.com/go-playground/validator/blob/v10.26.0/baked_in.go#L68C2-L68C16
	case "iscolor":
		verb := "must be a valid color (HEX, RGB, RGBA, HSL, or HSLA)"
		return verb, true
	case "country_code":
		verb := "must be a valid country code"
		return verb, true
	case "eu_country_code":
		verb := "must be a valid EU country code"
		return verb, true

	// bakedInValidators
	// https://github.com/go-playground/validator/blob/v10.26.0/baked_in.go#L77
	case "required":
		verb := "is required"
		return verb, true
	case "required_if":
		verb := "is required"
		return verb, true
	case "required_unless":
		verb := "is required"
		return verb, true
	case "skip_unless":
		verb := "is invalid"
		return verb, true
	case "required_with":
		verb := "is required"
		return verb, true
	case "required_with_all":
		verb := "is required"
		return verb, true
	case "required_without":
		verb := "is required"
		return verb, true
	case "required_without_all":
		verb := "is required"
		return verb, true
	case "excluded_if":
		verb := "must not be provided"
		return verb, true
	case "excluded_unless":
		verb := "must not be provided"
		return verb, true
	case "excluded_with":
		verb := "must not be provided"
		return verb, true
	case "excluded_with_all":
		verb := "must not be provided"
		return verb, true
	case "excluded_without":
		verb := "must not be provided"
		return verb, true
	case "excluded_without_all":
		verb := "must not be provided"
		return verb, true
	case "isdefault":
		verb := "must be the default value"
		return verb, true
	case "len":
		verb := "must have the required length"
		return verb, true
	case "min":
		verb := "must not be below the minimum allowed"
		return verb, true
	case "max":
		verb := "must not exceed the maximum allowed"
		return verb, true
	case "eq":
		verb := "must equal the specified value"
		return verb, true
	case "eq_ignore_case":
		verb := "must equal the specified value (case insensitive)"
		return verb, true
	case "ne":
		verb := "must not equal the specified value"
		return verb, true
	case "ne_ignore_case":
		verb := "must not equal the specified value (case insensitive)"
		return verb, true
	case "lt":
		verb := "must be less than the specified value"
		return verb, true
	case "lte":
		verb := "must be less than or equal to the specified value"
		return verb, true
	case "gt":
		verb := "must be greater than the specified value"
		return verb, true
	case "gte":
		verb := "must be greater than or equal to the specified value"
		return verb, true
	case "eqfield":
		verb := "must equal the other field's value"
		return verb, true
	case "eqcsfield":
		verb := "must equal the other field's value"
		return verb, true
	case "necsfield":
		verb := "must not equal the other field's value"
		return verb, true
	case "gtcsfield":
		verb := "must be greater than the other field's value"
		return verb, true
	case "gtecsfield":
		verb := "must be greater than or equal to the other field's value"
		return verb, true
	case "ltcsfield":
		verb := "must be less than the other field's value"
		return verb, true
	case "ltecsfield":
		verb := "must be less than or equal to the other field's value"
		return verb, true
	case "nefield":
		verb := "must not equal the other field's value"
		return verb, true
	case "gtefield":
		verb := "must be greater than or equal to the other field's value"
		return verb, true
	case "gtfield":
		verb := "must be greater than the other field's value"
		return verb, true
	case "ltefield":
		verb := "must be less than or equal to the other field's value"
		return verb, true
	case "ltfield":
		verb := "must be less than the other field's value"
		return verb, true
	case "fieldcontains":
		verb := "must contain the other field's value"
		return verb, true
	case "fieldexcludes":
		verb := "must not contain the other field's value"
		return verb, true
	case "alpha":
		verb := "must contain only letters"
		return verb, true
	case "alphanum":
		verb := "must contain only letters and numbers"
		return verb, true
	case "alphaunicode":
		verb := "must contain only letters"
		return verb, true
	case "alphanumunicode":
		verb := "must contain only letters and numbers"
		return verb, true
	case "boolean":
		verb := "must be a boolean (true or false)"
		return verb, true
	case "numeric":
		verb := "must contain only digits"
		return verb, true
	case "number":
		verb := "must be a valid number"
		return verb, true
	case "hexadecimal":
		verb := "must contain only hexadecimal characters"
		return verb, true
	case "hexcolor":
		verb := "must be a valid hex color code"
		return verb, true
	case "rgb":
		verb := "must be a valid RGB color"
		return verb, true
	case "rgba":
		verb := "must be a valid RGBA color"
		return verb, true
	case "hsl":
		verb := "must be a valid HSL color"
		return verb, true
	case "hsla":
		verb := "must be a valid HSLA color"
		return verb, true
	case "e164":
		verb := "must be a valid E.164 phone number"
		return verb, true
	case "email":
		verb := "must be a valid email address"
		return verb, true
	case "url":
		verb := "must be a valid URL"
		return verb, true
	case "http_url":
		verb := "must be a valid HTTP or HTTPS URL"
		return verb, true
	case "uri":
		verb := "must be a valid URI"
		return verb, true
	case "urn_rfc2141":
		verb := "must be a valid URN"
		return verb, true
	case "file":
		verb := "must be a valid file"
		return verb, true
	case "filepath":
		verb := "must be a valid file path"
		return verb, true
	case "base32":
		verb := "must be a valid base32 string"
		return verb, true
	case "base64":
		verb := "must be a valid base64 string"
		return verb, true
	case "base64url":
		verb := "must be a valid base64 URL-safe string"
		return verb, true
	case "base64rawurl":
		verb := "must be a valid base64 URL-safe string (unpadded)"
		return verb, true
	case "contains":
		verb := "must contain the specified substring"
		return verb, true
	case "containsany":
		verb := "must contain at least one of the specified characters"
		return verb, true
	case "containsrune":
		verb := "must contain the specified character"
		return verb, true
	case "excludes":
		verb := "must not contain the specified substring"
		return verb, true
	case "excludesall":
		verb := "must not contain any of the specified characters"
		return verb, true
	case "excludesrune":
		verb := "must not contain the specified character"
		return verb, true
	case "startswith":
		verb := "must start with the specified substring"
		return verb, true
	case "endswith":
		verb := "must end with the specified substring"
		return verb, true
	case "startsnotwith":
		verb := "must not start with the specified substring"
		return verb, true
	case "endsnotwith":
		verb := "must not end with the specified substring"
		return verb, true
	case "image":
		verb := "must be a valid image"
		return verb, true
	case "isbn":
		verb := "must be a valid ISBN"
		return verb, true
	case "isbn10":
		verb := "must be a valid ISBN-10"
		return verb, true
	case "isbn13":
		verb := "must be a valid ISBN-13"
		return verb, true
	case "issn":
		verb := "must be a valid ISSN"
		return verb, true
	case "eth_addr":
		verb := "must be a valid Ethereum address"
		return verb, true
	case "eth_addr_checksum":
		verb := "must be a valid Ethereum address with checksum"
		return verb, true
	case "btc_addr":
		verb := "must be a valid Bitcoin address"
		return verb, true
	case "btc_addr_bech32":
		verb := "must be a valid Bech32 Bitcoin address"
		return verb, true
	case "uuid":
		verb := "must be a valid UUID"
		return verb, true
	case "uuid3":
		verb := "must be a valid UUID version 3"
		return verb, true
	case "uuid4":
		verb := "must be a valid UUID version 4"
		return verb, true
	case "uuid5":
		verb := "must be a valid UUID version 5"
		return verb, true
	case "uuid_rfc4122":
		verb := "must be a valid RFC 4122 UUID"
		return verb, true
	case "uuid3_rfc4122":
		verb := "must be a valid RFC 4122 UUID version 3"
		return verb, true
	case "uuid4_rfc4122":
		verb := "must be a valid RFC 4122 UUID version 4"
		return verb, true
	case "uuid5_rfc4122":
		verb := "must be a valid RFC 4122 UUID version 5"
		return verb, true
	case "ulid":
		verb := "must be a valid ULID"
		return verb, true
	case "md4":
		verb := "must be a valid MD4 hash"
		return verb, true
	case "md5":
		verb := "must be a valid MD5 hash"
		return verb, true
	case "sha256":
		verb := "must be a valid SHA-256 hash"
		return verb, true
	case "sha384":
		verb := "must be a valid SHA-384 hash"
		return verb, true
	case "sha512":
		verb := "must be a valid SHA-512 hash"
		return verb, true
	case "ripemd128":
		verb := "must be a valid RIPEMD-128 hash"
		return verb, true
	case "ripemd160":
		verb := "must be a valid RIPEMD-160 hash"
		return verb, true
	case "tiger128":
		verb := "must be a valid TIGER-128 hash"
		return verb, true
	case "tiger160":
		verb := "must be a valid TIGER-160 hash"
		return verb, true
	case "tiger192":
		verb := "must be a valid TIGER-192 hash"
		return verb, true
	case "ascii":
		verb := "must contain only ASCII characters"
		return verb, true
	case "printascii":
		verb := "must contain only printable ASCII characters"
		return verb, true
	case "multibyte":
		verb := "must contain at least one multibyte character"
		return verb, true
	case "datauri":
		verb := "must be a valid data URI"
		return verb, true
	case "latitude":
		verb := "must be a valid latitude coordinate"
		return verb, true
	case "longitude":
		verb := "must be a valid longitude coordinate"
		return verb, true
	case "ssn":
		verb := "must be a valid SSN"
		return verb, true
	case "ipv4":
		verb := "must be a valid IPv4 address"
		return verb, true
	case "ipv6":
		verb := "must be a valid IPv6 address"
		return verb, true
	case "ip":
		verb := "must be a valid IP address"
		return verb, true
	case "cidrv4":
		verb := "must be a valid IPv4 CIDR"
		return verb, true
	case "cidrv6":
		verb := "must be a valid IPv6 CIDR"
		return verb, true
	case "cidr":
		verb := "must be a valid CIDR"
		return verb, true
	case "tcp4_addr":
		verb := "must be a valid IPv4 TCP address"
		return verb, true
	case "tcp6_addr":
		verb := "must be a valid IPv6 TCP address"
		return verb, true
	case "tcp_addr":
		verb := "must be a valid TCP address"
		return verb, true
	case "udp4_addr":
		verb := "must be a valid IPv4 UDP address"
		return verb, true
	case "udp6_addr":
		verb := "must be a valid IPv6 UDP address"
		return verb, true
	case "udp_addr":
		verb := "must be a valid UDP address"
		return verb, true
	case "ip4_addr":
		verb := "must be a resolvable IPv4 address"
		return verb, true
	case "ip6_addr":
		verb := "must be a resolvable IPv6 address"
		return verb, true
	case "ip_addr":
		verb := "must be a resolvable IP address"
		return verb, true
	case "unix_addr":
		verb := "must be a valid Unix socket address"
		return verb, true
	case "mac":
		verb := "must be a valid MAC address"
		return verb, true
	case "hostname":
		verb := "must be a valid hostname"
		return verb, true
	case "hostname_rfc1123":
		verb := "must be a valid hostname"
		return verb, true
	case "fqdn":
		verb := "must be a valid fully qualified domain name"
		return verb, true
	case "unique":
		verb := "must not contain duplicate values"
		return verb, true
	case "oneof":
		verb := "must be one of the allowed values"
		return verb, true
	case "oneofci":
		verb := "must be one of the allowed values (case insensitive)"
		return verb, true
	case "html":
		verb := "must be valid HTML"
		return verb, true
	case "html_encoded":
		verb := "must be a valid HTML-encoded string"
		return verb, true
	case "url_encoded":
		verb := "must be a valid URL-encoded string"
		return verb, true
	case "dir":
		verb := "must be a valid directory"
		return verb, true
	case "dirpath":
		verb := "must be a valid directory path"
		return verb, true
	case "json":
		verb := "must be valid JSON"
		return verb, true
	case "jwt":
		verb := "must be a valid JWT"
		return verb, true
	case "hostname_port":
		verb := "must include a valid hostname and port"
		return verb, true
	case "port":
		verb := "must be a valid port number"
		return verb, true
	case "lowercase":
		verb := "must be lowercase"
		return verb, true
	case "uppercase":
		verb := "must be uppercase"
		return verb, true
	case "datetime":
		verb := "must be a valid datetime"
		return verb, true
	case "timezone":
		verb := "must be a valid time zone"
		return verb, true
	case "iso3166_1_alpha2":
		verb := "must be a valid ISO 3166-1 alpha-2 country code"
		return verb, true
	case "iso3166_1_alpha2_eu":
		verb := "must be a valid ISO 3166-1 alpha-2 EU country code"
		return verb, true
	case "iso3166_1_alpha3":
		verb := "must be a valid ISO 3166-1 alpha-3 country code"
		return verb, true
	case "iso3166_1_alpha3_eu":
		verb := "must be a valid ISO 3166-1 alpha-3 EU country code"
		return verb, true
	case "iso3166_1_alpha_numeric":
		verb := "must be a valid ISO 3166-1 numeric country code"
		return verb, true
	case "iso3166_1_alpha_numeric_eu":
		verb := "must be a valid ISO 3166-1 numeric EU country code"
		return verb, true
	case "iso3166_2":
		verb := "must be a valid ISO 3166-2 code"
		return verb, true
	case "iso4217":
		verb := "must be a valid ISO 4217 currency code"
		return verb, true
	case "iso4217_numeric":
		verb := "must be a valid ISO 4217 numeric currency code"
		return verb, true
	case "bcp47_language_tag":
		verb := "must be a valid BCP 47 language tag"
		return verb, true
	case "postcode_iso3166_alpha2":
		verb := "must be a valid postal code for the specified country"
		return verb, true
	case "postcode_iso3166_alpha2_field":
		verb := "must be a valid postal code for the specified country"
		return verb, true
	case "bic":
		verb := "must be a valid BIC"
		return verb, true
	case "semver":
		verb := "must be a valid semantic version"
		return verb, true
	case "dns_rfc1035_label":
		verb := "must be a valid DNS label"
		return verb, true
	case "credit_card":
		verb := "must be a valid credit card number"
		return verb, true
	case "cve":
		verb := "must be a valid CVE identifier"
		return verb, true
	case "luhn_checksum":
		verb := "must have a valid Luhn checksum"
		return verb, true
	case "mongodb":
		verb := "must be a valid MongoDB object ID"
		return verb, true
	case "mongodb_connection_string":
		verb := "must be a valid MongoDB connection string"
		return verb, true
	case "cron":
		verb := "must be a valid cron expression"
		return verb, true
	case "spicedb":
		verb := "must be a valid SpiceDB identifier"
		return verb, true
	case "ein":
		verb := "must be a valid EIN"
		return verb, true
	case "validateFn":
		verb := "is invalid"
		return verb, true
	default:
		if p.logger != nil {
			p.logger.Error("unsupported validation pattern error", "tag", tag)
		}

		return "is invalid", false
	}
}
