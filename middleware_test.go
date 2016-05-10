package negronicloudwatch

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cw "github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/context"
	"github.com/stretchr/testify/assert"
)

var (
	nowTime  = time.Now()
	nowToday = nowTime.Format("2006-01-02")
)

type testClock struct{}

func (tc *testClock) Now() time.Time {
	return nowTime
}

func (tc *testClock) Since(time.Time) time.Duration {
	return 10 * time.Microsecond
}

func TestNewMiddleware_Name(t *testing.T) {
	mw := New("us-east-1", "test")
	assert.Equal(t, "test", mw.Namespace)
}

func TestNewMiddleware_LatencyMetricName(t *testing.T) {
	mw := New("us-east-1", "test")
	assert.Equal(t, "Latency", mw.LatencyMetricName)
}

func TestNewMiddleware_Service(t *testing.T) {
	mw := New("us-east-1", "test")
	assert.NotEqual(t, nil, mw.Service)
}

func setupServeHTTP(t *testing.T) (*Middleware, negroni.ResponseWriter, *http.Request) {
	req, err := http.NewRequest("GET", "http://example.com/stuff?rly=ya", nil)
	assert.Nil(t, err)

	req.RequestURI = "http://example.com/stuff?rly=ya"
	req.Method = "GET"
	req.Header.Set("X-Request-Id", "22035D08-98EF-413C-BBA0-C4E66A11B28D")
	req.Header.Set("X-Real-IP", "10.10.10.10")

	mw := New("us-east-1", "test")
	mw.clock = &testClock{}
	if err := mw.ExcludeURL("/ping"); err != nil {
		t.Fatalf("Can't exclude URL %q: %q", "/ping", err)
	}

	return mw, negroni.NewResponseWriter(httptest.NewRecorder()), req
}

func TestMiddleware_ServeHTTP(t *testing.T) {
	mw, rec, req := setupServeHTTP(t)
	mw.PutMetric = func(data []*cw.MetricDatum) {
		assert.Len(t, data, 1)
		assert.Len(t, data[0].Dimensions, 2)
		assert.Equal(t, "10.10.10.10", *data[0].Dimensions[1].Value)
		assert.Equal(t, "RemoteAddr", *data[0].Dimensions[1].Name)
		assert.Equal(t, "Latency", *data[0].MetricName)
		assert.Equal(t, "Microseconds", *data[0].Unit)
	}
	mw.ServeHTTP(rec, req, func(w http.ResponseWriter, r *http.Request) {
		val := context.Get(req, PutMetric)
		assert.NotEqual(t, nil, val)
		w.WriteHeader(418)
	})
}

func TestServeHTTPWithURLExcluded(t *testing.T) {
	mw, rec, req := setupServeHTTP(t)
	if err := mw.ExcludeURL(req.URL.Path); err != nil {
		t.Fatalf("Can't exclude URL %q: %q", "req.URL.Path", err)
	}

	called := false
	mw.PutMetric = func(data []*cw.MetricDatum) {
		called = true
	}

	mw.ServeHTTP(rec, req, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(418)
	})

	assert.Equal(t, false, called)
}

func TestRealClock_Now(t *testing.T) {
	rc := &realClock{}
	tf := "2006-01-02T15:04:05"
	assert.Equal(t, rc.Now().Format(tf), time.Now().Format(tf))
}

func TestRealClock_Since(t *testing.T) {
	rc := &realClock{}
	now := rc.Now()

	time.Sleep(10 * time.Millisecond)
	since := rc.Since(now)

	assert.Regexp(t, "^1[0-5]\\.[0-9]+ms", since.String())
}
