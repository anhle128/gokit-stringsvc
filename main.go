package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	mLog "log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-kit/kit/endpoint"
	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"

	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	httptransport "github.com/go-kit/kit/transport/http"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

type IStringService interface {
	Uppercase(context.Context, string) (string, error)
	Count(context.Context, string) (int, error)
}

type stringService struct{}

//
// ────────────────────────────────────────────────────────── I ──────────
//   :::::: U P P E R C A S E : :  :   :    :     :        :          :
// ────────────────────────────────────────────────────────────────────
//

type uppercaseRequest struct {
	S string `json:"s"`
}

type uppercaseResponse struct {
	V   string `json:"v"`
	Err string `json:"err,omitempty"`
}

func (stringService) Uppercase(ctx context.Context, s string) (string, error) {
	if s == "" {
		return "", errors.New("Empty string")
	}
	return strings.ToUpper(s), nil
}

func makeUppercaseEndpoint(svc IStringService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(uppercaseRequest)
		v, err := svc.Uppercase(ctx, req.S)
		if err != nil {
			return uppercaseResponse{"", err.Error()}, nil
		}
		return uppercaseResponse{v, ""}, nil
	}
}

func decodeUppercaseRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request uppercaseRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

//
// ────────────────────────────────────────────────── I ──────────
//   :::::: C O U N T : :  :   :    :     :        :          :
// ────────────────────────────────────────────────────────────
//

type countRequest struct {
	S string `json:"s"`
}

type countResponse struct {
	V   int    `json:"v"`
	Err string `json:"err,omitempty"`
}

func (stringService) Count(ctx context.Context, s string) (int, error) {
	if s == "" {
		return -1, errors.New("Empty string")
	}
	return len(s), nil
}

func makeCountEndpoint(svc IStringService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(countRequest)
		v, err := svc.Count(ctx, req.S)
		if err != nil {
			return countResponse{-1, err.Error()}, nil
		}
		return countResponse{v, ""}, nil
	}
}

func decodeCountRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request countRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

//
// ──────────────────────────────────────────────── I ──────────
//   :::::: M A I N : :  :   :    :     :        :          :
// ──────────────────────────────────────────────────────────
//

func main() {

	logger := log.NewLogfmtLogger(os.Stderr)

	fieldKeys := []string{"method", "error"}
	requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_count",
		Help:      "Number of requests received.",
	}, fieldKeys)
	requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_latency_microseconds",
		Help:      "Total duration of requests in microseconds.",
	}, fieldKeys)
	countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "count_result",
		Help:      "The result of each count method.",
	}, []string{}) // no fields here

	var svc IStringService
	svc = stringService{}
	svc = loggingMiddleware{logger, svc}
	svc = instrumentingMiddleware{requestCount, requestLatency, countResult, svc}

	uppercaseEndpoint := makeUppercaseEndpoint(svc)
	// uppercaseEndpoint = loggingMiddleware(logger)(uppercaseEndpoint)

	uppercaseHandler := httptransport.NewServer(
		uppercaseEndpoint,
		decodeUppercaseRequest,
		encodeResponse,
	)

	countEnpoint := makeCountEndpoint(svc)
	// countEnpoint = loggingMiddleware(logger)(countEnpoint)

	countHandler := httptransport.NewServer(
		countEnpoint,
		decodeCountRequest,
		encodeResponse,
	)

	http.Handle("/uppercase", uppercaseHandler)
	http.Handle("/count", countHandler)
	mLog.Fatal(http.ListenAndServe(":8080", nil))
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}

//
// ────────────────────────────────────────────────────────────── I ──────────
//   :::::: M I D D L E W A R E S : :  :   :    :     :        :          :
// ────────────────────────────────────────────────────────────────────────
//

//
// ─── LOGGING ────────────────────────────────────────────────────────────────────
//

type loggingMiddleware struct {
	logger log.Logger
	next   IStringService
}

func (mw loggingMiddleware) Uppercase(ctx context.Context, s string) (output string, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "uppercase",
			"input", s,
			"output", output,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	output, err = mw.next.Uppercase(ctx, s)
	return
}

func (mw loggingMiddleware) Count(ctx context.Context, s string) (n int, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "count",
			"input", s,
			"n", n,
			"took", time.Since(begin),
		)
	}(time.Now())
	n, err = mw.next.Count(ctx, s)
	return
}

// func loggingMiddleware(logger log.Logger) endpoint.Middleware {
// 	return func(next endpoint.Endpoint) endpoint.Endpoint {
// 		return func(ctx context.Context, request interface{}) (interface{}, error) {
// 			logger.Log("mgs", "calling endpoint")
// 			defer logger.Log("mgs", "called endpoint")
// 			return next(ctx, request)
// 		}
// 	}
// }

//
// ─── INSTRUMENTATION ────────────────────────────────────────────────────────────
//

type instrumentingMiddleware struct {
	requestCount   metrics.Counter
	requestLatency metrics.Histogram
	countResult    metrics.Histogram
	next           IStringService
}

func (mw instrumentingMiddleware) Uppercase(ctx context.Context, s string) (output string, err error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "uppercase", "error", fmt.Sprint(err != nil)}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
	}(time.Now())

	output, err = mw.next.Uppercase(ctx, s)
	return
}

func (mw instrumentingMiddleware) Count(ctx context.Context, s string) (n int, err error) {
	defer func(begin time.Time) {
		lvs := []string{"method", "count", "error", "false"}
		mw.requestCount.With(lvs...).Add(1)
		mw.requestLatency.With(lvs...).Observe(time.Since(begin).Seconds())
		mw.countResult.Observe(float64(n))
	}(time.Now())

	n, err = mw.next.Count(ctx, s)
	return
}
