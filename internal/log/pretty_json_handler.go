package log

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
)

type PrettyJSONHandlerOptions struct {
	slog.HandlerOptions
	PrettyPrint bool
}

func NewPrettyJSONHandler(w io.Writer, opts *PrettyJSONHandlerOptions) slog.Handler {
	if opts == nil {
		opts = &PrettyJSONHandlerOptions{}
	}

	return &prettyHandler{
		JSONHandler:    slog.NewJSONHandler(w, &opts.HandlerOptions),
		writer:         w,
		prettyPrint:    opts.PrettyPrint,
		handlerOptions: &opts.HandlerOptions,
	}
}

type prettyHandler struct {
	*slog.JSONHandler
	writer         io.Writer
	prettyPrint    bool
	handlerOptions *slog.HandlerOptions
}

func (h prettyHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.prettyPrint {
		return h.JSONHandler.Handle(ctx, r)
	}

	buf := &bytes.Buffer{}

	tempHandler := slog.NewJSONHandler(buf, h.handlerOptions)
	if err := tempHandler.Handle(ctx, r); err != nil {
		return err
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, buf.Bytes(), "", "  "); err != nil {
		return err
	}

	_, err := h.writer.Write(prettyJSON.Bytes())

	return err
}
