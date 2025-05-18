package cerrors

import (
	"fmt"
	"slices"

	crErrors "github.com/cockroachdb/errors"
)

// StackTraceの順番設定
// * Go的には、一般的には逆順（leaf-first） に並べる
//   - 例) エラーが発生したフレーム(newest)を先頭 → runtime.goexit (oldest) を最後
//
// * 一番古いフレームを先頭 (root-first) に並べる場合
//   - Sentry や一部の APM に JSON を送るとき
//   - Sentry の Stack Trace Interface は oldest → newest を要求
//   - Python、Node.js などのトレースバックは基本root-first
type StackTraceOrder string

const (
	StackTraceOrderOldestFirst StackTraceOrder = "oldest_first"
	StackTraceOrderNewestFirst StackTraceOrder = "newest_first"
)

var stackTraceOder = StackTraceOrderNewestFirst

func init() {
	// センチネル値 (sentinel) の検証
	// ErrorKind を追加したら ErrorKindCount が一つ下に移動し、その値が必ず増える
	// len(constructors)の値と比較することで、両者の値が違えば自動的にずれ(=constructors定義追加漏れ)を検知できる
	// init() でプログラム起動時（またはテスト時）に即座にパニックを起こし検知できる
	// 単体テストでも検知可能
	if len(constructors) != int(ErrorKindCount) {
		panic(fmt.Sprintf(
			"cerrors: constructors マップの要素数(%d) が ErrorKindCount(%d) と一致しません",
			len(constructors), ErrorKindCount,
		))
	}
}

func SetStackTraceOder(order StackTraceOrder) {
	stackTraceOder = order
}

// customErrorConstructor は CustomError を生成するコンストラクタオブジェクト
// New メソッドでオプションを適用して CustomError を返す
type customErrorConstructor struct {
	errCode string
	detail  string
}

// ErrorKind は enum風に定義されたエラー種別を表す
type errorKind int

// New は 事前に定義されたErrorKindの情報をもとに CustomError を作成し、Functional Options Pattern でカスタマイズして返す
func (ek errorKind) New(options ...Option) error {

	ctor, ok := constructors[ek]
	if !ok {
		return fmt.Errorf("invalid ErrorKind: %d", ek)
	}

	customError := &CustomError{
		errCode: ctor.errCode,
		detail:  ctor.detail,
	}

	// Functional Options Pattern でのオプションの処理
	for _, option := range options {
		option(customError)
	}

	// １. withstack でラップして生のスタックを取る
	wrapped := crErrors.WithStackDepth(customError, 1)

	// ２. 取れた ReportableStackTrace を自前フィールドに格納
	st := crErrors.GetReportableStackTrace(wrapped)
	frames := st.Frames
	if stackTraceOder == StackTraceOrderNewestFirst {
		slices.Reverse(frames)
	}
	customError.stack = frames

	// ３. CustomError 本体を返す（ここだけで stack 情報持ち回り）
	return customError
}

// ErrXxxの定義はすべてここで行う
const (
	// ErrUnknown は定義されていないエラー全般を表す
	ErrUnknown errorKind = iota

	// システム/インフラ関連
	ErrSystemInternal    // 内部システムエラー（初期化エラーを含む）
	ErrResourceExhausted // リソース枯渇
	ErrTimeout           // タイムアウト
	ErrUnavailable       // サービス利用不可

	// データベース関連
	ErrDBConnection // DB接続エラー
	ErrDBOperation  // DBオペレーションエラー
	ErrDBConstraint // DB制約違反
	ErrDBNotFound   // レコード未検出
	ErrDBDuplicate  // 重複レコード

	// API/HTTP関連
	ErrAPIRequest         // 不正なAPIリクエスト
	ErrAPIResponse        // APIレスポンスエラー
	ErrRateLimit          // レート制限超過
	ErrServiceUnavailable // 外部サービス利用不可

	// 認証/認可関連
	ErrAuthentication // 認証エラー
	ErrAuthorization  // 認可エラー
	ErrTokenExpired   // トークン期限切れ
	ErrTokenInvalid   // 不正なトークン

	// バリデーション関連
	ErrValidation    // バリデーションエラー（無効な入力を含む）
	ErrInvalidFormat // フォーマットエラー
	ErrMissingField  // 必須フィールド欠落
	ErrInvalidState  // 不正な状態

	// ビジネスロジック関連
	ErrBusinessRule     // ビジネスルール違反
	ErrOperationFailed  // 操作失敗
	ErrInvalidOperation // 不正な操作
	ErrResourceNotFound // リソース未検出

	// --- 新しい ErrorKind は常にこの上↑に追加する ---
	//
	// ErrorKindCount はセンチネル値(sentinel/配列やリストの終端を示す特別な値)であって、上記の要素数として使うためのもので、
	// 最下部にあることで iota によって、要素が追加されるたびにその値が増える.
	// この一番最下部にあるべきものである ErrorKindCount の値と len(constructors) の値をチェックすることで自動的にずれを検知できる
	// ずれ = constructorsへの定義追加漏れ
	ErrorKindCount
)

// constructors は各 ErrorKind に対するカスタムエラーコンストラクタをキー付きで保持する
var constructors = map[errorKind]customErrorConstructor{
	// 基本エラー
	ErrUnknown: {"UNKNOWN_ERROR", "an unexpected error occurred"}, // "予期せぬエラーが発生

	// システム/インフラ関連
	ErrSystemInternal:    {"SYSTEM_INTERNAL", "internal system error occurred"},          // 内部システムエラーが発生
	ErrResourceExhausted: {"RESOURCE_EXHAUSTED", "system resources have been exhausted"}, // リソースが枯渇
	ErrTimeout:           {"TIMEOUT", "operation timed out"},                             // タイムアウトが発生
	ErrUnavailable:       {"UNAVAILABLE", "service is currently unavailable"},            // サービスが利用不可

	// データベース関連
	ErrDBConnection: {"DB_CONNECTION", "failed to establish database connection"}, // データベース接続エラー
	ErrDBOperation:  {"DB_OPERATION", "database operation failed"},                // データベース操作エラー
	ErrDBConstraint: {"DB_CONSTRAINT", "database constraint violation occurred"},  // データベース制約違反
	ErrDBNotFound:   {"DB_NOT_FOUND", "requested record not found in database"},   // レコード該当なし
	ErrDBDuplicate:  {"DB_DUPLICATE", "duplicate record detected in database"},    // レコードが重複

	// API/HTTP関連
	ErrAPIRequest:         {"API_REQUEST", "invalid API request"},                     // 不正なAPIリクエスト
	ErrAPIResponse:        {"API_RESPONSE", "API response error occurred"},            // APIレスポンスエラー
	ErrRateLimit:          {"RATE_LIMIT", "rate limit exceeded"},                      // レート制限を超過
	ErrServiceUnavailable: {"SERVICE_UNAVAILABLE", "external service is unavailable"}, // 外部サービスが利用不可

	// 認証/認可関連
	ErrAuthentication: {"AUTHENTICATION", "authentication failed"},           // 認証エラー
	ErrAuthorization:  {"AUTHORIZATION", "authorization failed"},             // 認可エラー
	ErrTokenExpired:   {"TOKEN_EXPIRED", "authentication token has expired"}, // トークンの有効期限が切れ
	ErrTokenInvalid:   {"TOKEN_INVALID", "invalid authentication token"},     // トークンが無効

	// バリデーション関連
	ErrValidation:    {"VALIDATION", "validation error occurred"},    // バリデーションエラーが発生
	ErrInvalidFormat: {"INVALID_FORMAT", "invalid format detected"},  //  フォーマットが不正
	ErrMissingField:  {"MISSING_FIELD", "required field is missing"}, // 必須フィールドが欠落
	ErrInvalidState:  {"INVALID_STATE", "invalid state detected"},    // 不正な状態

	// ビジネスロジック関連
	ErrBusinessRule:     {"BUSINESS_RULE", "business rule violation occurred"},  // ビジネスルール違反が発生
	ErrOperationFailed:  {"OPERATION_FAILED", "operation failed to complete"},   // 操作が失敗
	ErrInvalidOperation: {"INVALID_OPERATION", "invalid operation attempted"},   // 不正な操作
	ErrResourceNotFound: {"RESOURCE_NOT_FOUND", "requested resource not found"}, // リソースなし
}
