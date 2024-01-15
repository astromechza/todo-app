package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/oapi-codegen/echo-middleware"

	"github.com/astromechza/todo-app/backend/model"
)

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config=oapi-codegen.cfg.yaml ../api.yaml

type Server struct {
	Database model.Modelling
}

func (s *Server) GetHealthZ(ctx context.Context, _ GetHealthZRequestObject) (GetHealthZResponseObject, error) {
	if err := s.Database.HealthZ(ctx); err != nil {
		return nil, err
	}
	return GetHealthZ200JSONResponse{}, nil
}

func DefaultErrorHandler(err error, c echo.Context) {
	requestId := uuid.NewString()
	problemUri := "request-id:" + requestId
	logger := slog.With("request-id", requestId, "method", c.Request().Method, "url", c.Request().URL.String())

	if e := model.ErrBadRequest(""); errors.As(err, &e) {
		if err = c.JSON(http.StatusBadRequest, StandardProblemResponse{
			Type:     "about:blank",
			Instance: &problemUri,
			Status:   http.StatusBadRequest,
			Title:    "Bad request",
			Detail:   e.Error(),
		}); err != nil {
			logger.Warn("failed to write default error response", "err", err)
		}
		return
	}

	if e := model.ErrNotFound(""); errors.As(err, &e) {
		if err = c.JSON(http.StatusNotFound, StandardProblemResponse{
			Type:     "about:blank",
			Instance: &problemUri,
			Status:   http.StatusNotFound,
			Title:    "Not found",
			Detail:   e.Error(),
		}); err != nil {
			logger.Warn("failed to write default error response", "err", err)
		}
		return
	}

	if errors.Is(err, echo.ErrNotFound) {
		if err = c.JSON(http.StatusNotFound, StandardProblemResponse{
			Type:     "about:blank",
			Instance: &problemUri,
			Status:   http.StatusNotFound,
			Title:    "Route not found",
			Detail:   "The requested HTTP URL was not routed to a known path. Please check the api documentation.",
		}); err != nil {
			logger.Warn("failed to write default error response", "err", err)
		}
		return
	}

	if errors.Is(err, echo.ErrMethodNotAllowed) {
		if err = c.JSON(http.StatusMethodNotAllowed, StandardProblemResponse{
			Type:     "about:blank",
			Instance: &problemUri,
			Status:   http.StatusMethodNotAllowed,
			Title:    "Method not allowed",
			Detail:   "The requested HTTP URL was routed but the request method is not supported. Please check the api documentation.",
		}); err != nil {
			logger.Warn("failed to write default error response", "err", err)
		}
		return
	}

	logger.Error("handling error in default error handler", "err", err)
	if !c.Response().Committed {
		if err = c.JSON(http.StatusInternalServerError, StandardProblemResponse{
			Type:     "about:blank",
			Instance: &problemUri,
			Status:   http.StatusInternalServerError,
			Title:    "An internal error occurred",
			Detail:   "An error occurred while processing the request or generating the response. Please validate and retry the request if necessary.",
		}); err != nil {
			logger.Warn("failed to write default error response", "err", err)
		}
	}
}

type DefaultJsonSerializer struct {
}

func (d *DefaultJsonSerializer) Serialize(c echo.Context, i interface{}, indent string) error {
	enc := json.NewEncoder(c.Response())
	if indent != "" {
		enc.SetIndent("", indent)
	}
	enc.SetEscapeHTML(false)
	return enc.Encode(i)
}

func (d *DefaultJsonSerializer) Deserialize(c echo.Context, i interface{}) error {
	req := c.Request()

	if req.Body == nil {
		return NewErrCouldNotParseRequest("request body is empty", nil)
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return NewErrCouldNotParseRequest("failed to read the raw request body", err)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	if err := dec.Decode(i); err != nil {
		var jute *json.UnmarshalTypeError
		if errors.As(err, &jute) {
			return NewErrCouldNotParseRequest(fmt.Sprintf("type error for field '%v' at offset %v, expected %v", jute.Field, jute.Offset, jute.Type), err)
		}
		var jse *json.SyntaxError
		if errors.As(err, &jse) {
			return NewErrCouldNotParseRequest(fmt.Sprintf("syntax error at offset %v: %v", jse.Offset, jse.Error()), err)
		}
		return NewErrCouldNotParseRequest("json parsing failed", err)
	}
	return nil
}

func BuildOpenApiValidator(raw []byte) (echo.MiddlewareFunc, error) {
	data, err := openapi3.NewLoader().LoadFromData(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to load open api schema: %w", err)
	}
	validator := echomiddleware.OapiRequestValidatorWithOptions(data, &echomiddleware.Options{
		ErrorHandler: func(c echo.Context, err *echo.HTTPError) error {
			switch err.Code {
			case http.StatusBadRequest:
				if stringMessage, ok := err.Message.(string); ok {
					return model.ErrBadRequest(stringMessage)
				}
			}
			return fmt.Errorf("unexpected error from openapi validator: %w", err)
		},
	})
	return validator, nil
}
