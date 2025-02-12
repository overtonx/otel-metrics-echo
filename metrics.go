package otelmetricsecho

import (
	"errors"
	"net/http"
	"strings"
	"time"

	semconv "go.opentelemetry.io/otel/semconv/v1.23.0"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	_           = iota // ignore first value by assigning to blank identifier
	bKB float64 = 1 << (10 * iota)
	bMB
)

const defaultServiceName = "echo"
const meterName = "otel_metrics_echo"

const (
	metricHTTPRequestsTotal          = "requests_total"
	metricHTTPRequestDurationSeconds = "request_duration_seconds"
	metricHTTPResponseSizeBytes      = "response_size_bytes"
	metricHTTPRequestSizeBytes       = "request_size_bytes"
)

var sizeBuckets = []float64{1.0 * bKB, 2.0 * bKB, 5.0 * bKB, 10.0 * bKB, 100 * bKB, 500 * bKB, 1.0 * bMB, 2.5 * bMB, 5.0 * bMB, 10.0 * bMB}

type MiddlewareConfig struct {
	// Skipper defines a function to skip middleware.
	Skipper                   middleware.Skipper
	ServiceName               string
	LabelFuncs                map[string]LabelValueFunc
	timeNow                   func() time.Time
	DoNotUseRequestPathFor404 bool
}

type LabelValueFunc func(c echo.Context, err error) string

func NewMiddleware(serviceName string) echo.MiddlewareFunc {
	return NewMiddlewareWithConfig(MiddlewareConfig{
		ServiceName: serviceName,
	})
}

func NewMiddlewareWithConfig(config MiddlewareConfig) echo.MiddlewareFunc {
	mw, err := config.ToMiddleware()
	if err != nil {
		panic(err)
	}

	return mw
}

func (conf MiddlewareConfig) ToMiddleware() (echo.MiddlewareFunc, error) {
	var meterProvider = otel.GetMeterProvider()
	metrics := meterProvider.Meter(meterName)

	if conf.timeNow == nil {
		conf.timeNow = time.Now
	}

	if conf.ServiceName == "" {
		conf.ServiceName = defaultServiceName
	}

	requestCount, _ := metrics.Int64Counter(
		metricHTTPRequestsTotal,
		metric.WithDescription("How many HTTP requests processed, partitioned by status code and HTTP method."),
	)

	requestDuration, _ := metrics.Float64Histogram(
		metricHTTPRequestDurationSeconds,
		metric.WithDescription("The HTTP request latencies in seconds."),
	)

	responseSize, _ := metrics.Float64Histogram(
		metricHTTPResponseSizeBytes,
		metric.WithDescription("The HTTP response sizes in bytes."),
		metric.WithExplicitBucketBoundaries(sizeBuckets...),
	)
	requestSize, _ := metrics.Float64Histogram(
		metricHTTPRequestSizeBytes,
		metric.WithDescription("The HTTP request sizes in bytes."),
		metric.WithExplicitBucketBoundaries(sizeBuckets...),
	)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if conf.Skipper != nil && conf.Skipper(c) {
				return next(c)
			}

			reqSz := computeApproximateRequestSize(c.Request())

			start := conf.timeNow()
			err := next(c)
			elapsed := float64(conf.timeNow().Sub(start)) / float64(time.Second)

			url := c.Path() // contains route path ala `/users/:id`
			if url == "" && !conf.DoNotUseRequestPathFor404 {
				// as of Echo v4.10.1 path is empty for 404 cases (when router did not find any matching routes)
				// in this case we use actual path from request to have some distinction in Prometheus
				url = c.Request().URL.Path
			}

			status := c.Response().Status
			if err != nil {
				var httpError *echo.HTTPError
				if errors.As(err, &httpError) {
					status = httpError.Code
				}
				if status == 0 || status == http.StatusOK {
					status = http.StatusInternalServerError
				}
			}

			var attrs []attribute.KeyValue
			attrs = append(attrs, semconv.ServiceName(conf.ServiceName))
			attrs = append(attrs, semconv.HTTPRoute(strings.ToValidUTF8(url, "\uFFFD")))
			attrs = append(attrs, semconv.HTTPRequestMethodKey.String(c.Request().Method))
			attrs = append(attrs, semconv.HostName(c.Scheme()))

			attrs = append(attrs, semconv.HTTPStatusCodeKey.Int(status))
			attrs = append(attrs, semconv.HTTPResponseStatusCode(status))

			for key, labelFunc := range conf.LabelFuncs {
				attrs = append(attrs, attribute.String(key, labelFunc(c, err)))
			}

			attributes := metric.WithAttributes(attrs...)

			ctx := c.Request().Context()

			requestCount.Add(ctx, 1, attributes)
			requestSize.Record(ctx, float64(reqSz), attributes)
			responseSize.Record(ctx, float64(c.Response().Size), attributes)
			requestDuration.Record(ctx, elapsed, attributes)

			return err
		}
	}, nil
}

func computeApproximateRequestSize(r *http.Request) int {
	s := 0
	if r.URL != nil {
		s = len(r.URL.Path)
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}

	return s
}
