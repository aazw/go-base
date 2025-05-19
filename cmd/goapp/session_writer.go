package main

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
)

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
