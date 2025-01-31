# otelmetricsecho

`otelmetricsecho` is a middleware for the [Echo](https://github.com/labstack/echo) web framework that integrates OpenTelemetry metrics for monitoring HTTP requests. It provides various metrics, including request count, duration, response size, and request size, helping with observability and performance tracking.

## Features
- Tracks HTTP request count, duration, request size, and response size.
- Supports configurable label functions for custom metric attributes.
- Uses OpenTelemetry for metric instrumentation.
- Handles 404 and other error responses gracefully.

## Installation
```sh
go get github.com/overtonx/otelmetricsecho
```

## Usage
### Basic Middleware Setup
```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/overtonx/otel-metrics-echo"
)

func main() {
	e := echo.New()
	e.Use(otelmetricsecho.NewMiddleware("myapp"))

	e.GET("/hello", func(c echo.Context) error {
		return c.String(200, "Hello, World!")
	})

	e.Start(":8080")
}
```

### Custom Middleware Configuration
```go
	config := otelmetricsecho.MiddlewareConfig{
		DoNotUseRequestPathFor404: true,
	}

	e.Use(otelmetricsecho.NewMiddlewareWithConfig(config))
````
## LabelFuncs
The `LabelFuncs` map allows you to append custom labels to the generated metrics by defining functions that extract values from the request context. This can be useful for adding custom metadata to your metrics, such as tenant IDs, user roles, or feature flags.

### Example: Adding Custom Metrics Labels
```go
config := otelmetricsecho.MiddlewareConfig{
	LabelFuncs: map[string]otelmetricsecho.LabelValueFunc{
		"tenant_id": func(c echo.Context, err error) string {
			return c.Request().Header.Get("X-Tenant-ID")
		},
		"user_role": func(c echo.Context, err error) string {
			return c.Request().Header.Get("X-User-Role")
		},
	},
}

e.Use(otelmetricsecho.NewMiddlewareWithConfig(config))
```
This configuration ensures that every metric emitted by the middleware contains additional labels `tenant_id` and `user_role` extracted from the request headers.

## Metrics Provided
### Request Count
- **Metric Name:** `requests_total`
- **Description:** Number of HTTP requests processed, partitioned by status code and method.

### Request Duration
- **Metric Name:** `request_duration_seconds`
- **Description:** The HTTP request latencies in seconds.

### Request Size
- **Metric Name:** `request_size_bytes`
- **Description:** The HTTP request sizes in bytes.

### Response Size
- **Metric Name:** `response_size_bytes`
- **Description:** The HTTP response sizes in bytes.
