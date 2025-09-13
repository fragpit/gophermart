package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mock_handlers "github.com/fragpit/gophermart/internal/api/handlers/mocks"
	"github.com/fragpit/gophermart/internal/model"
	"go.uber.org/mock/gomock"
)

func TestNewAuthRegisterHandler(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	type returnData struct {
		token string
		err   error
	}

	tests := []struct {
		name           string
		returnData     *returnData
		reqBody        *authRequest
		contentType    string
		wantCode       int
		wantBodySubstr string
		wantAuthHeader string
	}{
		{
			name: "success",
			returnData: &returnData{
				token: "tok123",
				err:   nil,
			},
			reqBody: &authRequest{
				Login:    "u",
				Password: "p",
			},
			wantCode:       http.StatusOK,
			wantBodySubstr: `{"token":"tok123"}`,
			wantAuthHeader: "Bearer tok123",
		},
		{
			name: "user exists",
			returnData: &returnData{
				token: "",
				err:   model.ErrUserExists,
			},
			reqBody: &authRequest{
				Login:    "",
				Password: "p",
			},
			wantCode:       http.StatusConflict,
			wantBodySubstr: http.StatusText(http.StatusConflict),
		},
		{
			name: "password policy violated",
			returnData: &returnData{
				token: "",
				err:   model.ErrPasswordPolicyViolated,
			},
			reqBody: &authRequest{
				Login:    "u",
				Password: "p",
			},
			wantCode:       http.StatusBadRequest,
			wantBodySubstr: "password policy violated",
		},
		{
			name: "internal error",
			returnData: &returnData{
				token: "",
				err:   fmt.Errorf("db down"),
			},
			reqBody: &authRequest{
				Login:    "u",
				Password: "p",
			},
			wantCode:       http.StatusInternalServerError,
			wantBodySubstr: http.StatusText(http.StatusInternalServerError),
		},
		{
			name: "wrong content type",
			returnData: &returnData{
				token: "",
				err:   nil,
			},
			reqBody: &authRequest{
				Login:    "u",
				Password: "p",
			},
			contentType:    "wrong/type",
			wantCode:       http.StatusUnsupportedMediaType,
			wantBodySubstr: http.StatusText(http.StatusUnsupportedMediaType),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mock_handlers.NewMockAuthService(ctrl)

			m.EXPECT().
				Register(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tc.returnData.token, tc.returnData.err).AnyTimes()
			handler := NewAuthRegisterHandler(m)
			rec := httptest.NewRecorder()

			if tc.contentType == "" {
				tc.contentType = "application/json"
			}

			b, _ := json.Marshal(tc.reqBody)
			req := httptest.NewRequest(
				http.MethodPost,
				"/",
				strings.NewReader(string(b)),
			)
			req.Header.Set("Content-Type", tc.contentType)

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf(
					"unexpected status code: got %d want %d",
					rec.Code,
					tc.wantCode,
				)
			}

			body := rec.Body.String()
			if !strings.Contains(body, tc.wantBodySubstr) {
				t.Fatalf(
					"unexpected body: got %q want substring %q",
					body,
					tc.wantBodySubstr,
				)
			}

			if tc.wantAuthHeader != "" {
				got := rec.Header().Get("Authorization")
				if got != tc.wantAuthHeader {
					t.Fatalf(
						"unexpected Authorization header: got %q want %q",
						got,
						tc.wantAuthHeader,
					)
				}
				ct := rec.Header().Get("Content-Type")
				if ct != "application/json" {
					t.Fatalf(
						"unexpected Content-Type: got %q want %q",
						ct,
						"application/json",
					)
				}
			}
		})
	}
}

func TestNewAuthLoginHandler(t *testing.T) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	type returnData struct {
		token string
		err   error
	}

	tests := []struct {
		name           string
		returnData     *returnData
		reqBody        *authRequest
		contentType    string
		wantCode       int
		wantBodySubstr string
		wantAuthHeader string
	}{
		{
			name: "success",
			returnData: &returnData{
				token: "tok123",
				err:   nil,
			},
			reqBody: &authRequest{
				Login:    "u",
				Password: "p",
			},
			wantCode:       http.StatusOK,
			wantBodySubstr: `{"token":"tok123"}`,
			wantAuthHeader: "Bearer tok123",
		},
		{
			name: "wrong username or password",
			returnData: &returnData{
				token: "",
				err:   model.ErrInvalidCredentials,
			},
			reqBody: &authRequest{
				Login:    "u",
				Password: "p",
			},
			wantCode:       http.StatusUnauthorized,
			wantBodySubstr: "wrong username or password",
		},
		{
			name: "internal error",
			returnData: &returnData{
				token: "",
				err:   fmt.Errorf("db down"),
			},
			reqBody: &authRequest{
				Login:    "u",
				Password: "p",
			},
			wantCode:       http.StatusInternalServerError,
			wantBodySubstr: http.StatusText(http.StatusInternalServerError),
		},
		{
			name: "wrong content type",
			returnData: &returnData{
				token: "",
				err:   nil,
			},
			reqBody: &authRequest{
				Login:    "u",
				Password: "p",
			},
			contentType:    "wrong/type",
			wantCode:       http.StatusUnsupportedMediaType,
			wantBodySubstr: http.StatusText(http.StatusUnsupportedMediaType),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mock_handlers.NewMockAuthService(ctrl)

			m.EXPECT().
				Login(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tc.returnData.token, tc.returnData.err).AnyTimes()
			handler := NewAuthLoginHandler(m)
			rec := httptest.NewRecorder()

			if tc.contentType == "" {
				tc.contentType = "application/json"
			}

			b, _ := json.Marshal(tc.reqBody)
			req := httptest.NewRequest(
				http.MethodPost,
				"/",
				strings.NewReader(string(b)),
			)
			req.Header.Set("Content-Type", tc.contentType)

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf(
					"unexpected status code: got %d want %d",
					rec.Code,
					tc.wantCode,
				)
			}

			body := rec.Body.String()
			if !strings.Contains(body, tc.wantBodySubstr) {
				t.Fatalf(
					"unexpected body: got %q want substring %q",
					body,
					tc.wantBodySubstr,
				)
			}

			if tc.wantAuthHeader != "" {
				got := rec.Header().Get("Authorization")
				if got != tc.wantAuthHeader {
					t.Fatalf(
						"unexpected Authorization header: got %q want %q",
						got,
						tc.wantAuthHeader,
					)
				}
				ct := rec.Header().Get("Content-Type")
				if ct != "application/json" {
					t.Fatalf(
						"unexpected Content-Type: got %q want %q",
						ct,
						"application/json",
					)
				}
			}
		})
	}
}
