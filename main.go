package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-kit/kit/endpoint"

	httptransport "github.com/go-kit/kit/transport/http"
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
	svc := stringService{}

	uppercaseHandler := httptransport.NewServer(
		makeUppercaseEndpoint(svc),
		decodeUppercaseRequest,
		encodeResponse,
	)

	countHandler := httptransport.NewServer(
		makeCountEndpoint(svc),
		decodeCountRequest,
		encodeResponse,
	)

	http.Handle("/uppercase", uppercaseHandler)
	http.Handle("/count", countHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))

}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}
