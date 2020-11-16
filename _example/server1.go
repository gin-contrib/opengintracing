package main

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/opengintracing"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/zipkin"
)

func main() {
	// Configure tracing
	propagator := zipkin.NewZipkinB3HTTPHeaderPropagator()
	trace, closer := jaeger.NewTracer(
		"server01",
		jaeger.NewConstSampler(true),
		jaeger.NewNullReporter(),
		jaeger.TracerOptions.Injector(opentracing.HTTPHeaders, propagator),
		jaeger.TracerOptions.Extractor(opentracing.HTTPHeaders, propagator),
		jaeger.TracerOptions.ZipkinSharedRPCSpan(true),
	)
	defer closer.Close()
	opentracing.SetGlobalTracer(trace)
	var fn opengintracing.ParentSpanReferenceFunc
	fn = func(sc opentracing.SpanContext) opentracing.StartSpanOption {
		return opentracing.ChildOf(sc)
	}

	// Set up routes
	r := gin.Default()
	r.POST("",
		opengintracing.SpanFromHeadersHttpFmt(trace, "service1", fn, false),
		handler)
	r.Run(":8001")
}

func printHeaders(message string, header http.Header) {
	fmt.Println(message)
	for k, v := range header {
		fmt.Printf("%s: %s\n", k, v)
	}
}

func handler(c *gin.Context) {
	span, found := opengintracing.GetSpan(c)
	if found == false {
		fmt.Println("Span not found")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	req, _ := http.NewRequest("POST", "http://localhost:8003", nil)

	opentracing.GlobalTracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	printHeaders("Incoming headers", c.Request.Header)
	printHeaders("Outgoing headers", req.Header)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Unexpected response from service3")
	}
	c.Status(http.StatusOK)
}
