package handler

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"strings"

	"github.com/envoyproxy/xds-relay/internal/pkg/log"
	"github.com/envoyproxy/xds-relay/internal/pkg/log/zap"

	"github.com/envoyproxy/xds-relay/internal/pkg/util/stringify"

	"github.com/envoyproxy/xds-relay/internal/app/orchestrator"

	bootstrapv1 "github.com/envoyproxy/xds-relay/pkg/api/bootstrap/v1"
)

type Handler struct {
	pattern     string
	description string
	handler     http.HandlerFunc
	redirect    bool
}

const (
	logURL   = "/log_level"
	cacheURL = "/cache"
	clearURL = "/cache/clear"
)

func getHandlers(bootstrap *bootstrapv1.Bootstrap,
	orchestrator *orchestrator.Orchestrator,
	weboff chan bool,
	logger log.Logger) []Handler {
	handlers := []Handler{
		{
			"/",
			"admin home page",
			func(http.ResponseWriter, *http.Request) {},
			true,
		},
		{
			"/ready",
			"ready endpoint. usage: GET /ready POST /ready/true or ready/false",
			readyHandler(weboff),
			true,
		},
		{
			clearURL,
			"clear cache entry for a given key. Omitting the key clears all cache entries. usage: `/cache/clear/<key>`",
			clearCacheHandler(orchestrator),
			true,
		},
		{
			cacheURL,
			"print cache entry for a given key. Omitting the key outputs all cache entries. usage: `/cache/<key>`",
			cacheDumpHandler(orchestrator),
			true,
		},
		{
			"/cache/version",
			"print the version for a particular key. usage: `/cache/version/<key>`",
			versionHandler(orchestrator),
			true,
		},
		{
			"/cache/eds",
			"print the eds payload for a particular key. usage: `/cache/eds/<key>`",
			edsDumpHandler(orchestrator),
			true,
		},
		{
			"/cache/keys",
			"print all keys",
			keyDumpHandler(orchestrator),
			true,
		},
		{
			logURL,
			"update the log level to `debug`, `info`, `warn`, or `error`. " +
				"Omitting the level outputs the current log level. usage: `/log_level/<level>`",
			logLevelHandler(logger),
			true,
		},
		{
			"/server_info",
			"print bootstrap configuration",
			configDumpHandler(bootstrap),
			true,
		},
		{
			"/debug/pprof/goroutine",
			"Stack traces of all current goroutines",
			pprof.Handler("goroutine").ServeHTTP,
			false,
		},
		{
			"/debug/pprof/heap",
			"A sampling of memory allocations of live objects.",
			pprof.Handler("heap").ServeHTTP,
			false,
		},
		{
			"/debug/pprof/threadcreate",
			"Stack traces that led to the creation of new OS threads",
			pprof.Handler("threadcreate").ServeHTTP,
			false,
		},
		{
			"/debug/pprof/block",
			"Stack traces that led to blocking on synchronization primitives",
			pprof.Handler("block").ServeHTTP,
			false,
		},
	}
	// The default handler is defined later to avoid infinite recursion.
	handlers[0].handler = defaultHandler(handlers)
	return handlers
}

func RegisterHandlers(bootstrapConfig *bootstrapv1.Bootstrap,
	orchestrator *orchestrator.Orchestrator,
	weboff chan bool,
	logger log.Logger) {
	for _, handler := range getHandlers(bootstrapConfig, orchestrator, weboff, logger) {
		http.Handle(handler.pattern, handler.handler)
		if !strings.HasSuffix(handler.pattern, "/") && handler.redirect {
			http.Handle(handler.pattern+"/", handler.handler)
		}
	}
}

func defaultHandler(handlers []Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// The "/" pattern matches everything, so we need to check
		// that we're at the root here.
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		fmt.Fprintf(w, "admin commands are:\n")
		for _, handler := range handlers {
			fmt.Fprintf(w, "  %s: %s\n", handler.pattern, handler.description)
		}
	}
}

func configDumpHandler(bootstrapConfig *bootstrapv1.Bootstrap) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		configString, err := stringify.InterfaceToString(bootstrapConfig)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Failed to dump config: %s\n", err.Error())
			return
		}
		fmt.Fprintf(w, "%s\n", configString)
	}
}

func getParam(path string, prefix string) string {
	path = strings.TrimPrefix(path, prefix)
	_, param := filepath.Split(path)
	return param
}

func logLevelHandler(l log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "POST" {
			logLevel := getParam(req.URL.Path, logURL)

			// If no key is provided, output the current log level.
			if logLevel == "" {
				fmt.Fprintf(w, "Current log level: %s\n", l.GetLevel())
				return
			}

			// Otherwise update the logging level.
			_, parseLogLevelErr := zap.ParseLogLevel(logLevel)
			if parseLogLevelErr != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Invalid log level: %s\n", logLevel)
				return
			}
			l.UpdateLogLevel(logLevel)
			fmt.Fprintf(w, "Current log level: %s\n", l.GetLevel())
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}
