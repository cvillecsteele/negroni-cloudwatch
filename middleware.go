// NEGRONI-CLOUDWATCH: Middleware for recording data to AWS Cloudwatch.
// Copyright (C) 2016, Colin Steele
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//   the Free Software Foundation, either version 3 of the License, or
//                  (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//              GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package negronicloudwatch

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	cw "github.com/aws/aws-sdk-go/service/cloudwatch"
)

type key int

const PutMetric key = 0

// Middleware handler
type Middleware struct {
	// CW Service
	Service *cw.CloudWatch

	// CW namespace
	Namespace string

	// Latency metric name
	LatencyMetricName string

	Before    BeforeFunc
	After     AfterFunc
	PutMetric func(data []*cw.MetricDatum)

	clock timer

	// Exclude URLs from logging
	excludeURLs map[string]struct{}
}

func New(region, namespace string) *Middleware {
	m := Middleware{
		Service: cw.New(session.New(), aws.NewConfig().WithRegion(region).WithMaxRetries(5)),

		Namespace:         namespace,
		LatencyMetricName: "Latency",
		Before:            DefaultBefore,
		After:             DefaultAfter,

		clock: &realClock{},

		excludeURLs: map[string]struct{}{},
	}
	m.PutMetric = func(data []*cw.MetricDatum) {
		putMetric(&m, data)
	}
	return &m
}

//
//
//
// []*cw.MetricDatum{
//       {
//         MetricName: aws.String("MetricName"),
//         Dimensions: []*cw.Dimension{
//           {
//             Name:  aws.String("DimensionName"),
//             Value: aws.String("DimensionValue"),
//           },
//         },
//         StatisticValues: &cw.StatisticSet{
//           Maximum:     aws.Float64(1.0),
//           Minimum:     aws.Float64(1.0),
//           SampleCount: aws.Float64(1.0),
//           Sum:         aws.Float64(1.0),
//         },
//         Timestamp: aws.Time(time.Now()),
//         Unit:      aws.String("StandardUnit"),
//         Value:     aws.Float64(1.0),
//       },
// }
func putMetric(m *Middleware, data []*cw.MetricDatum) {
	params := &cw.PutMetricDataInput{
		MetricData: data,
		Namespace:  aws.String(m.Namespace),
	}
	_, err := m.Service.PutMetricData(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			fmt.Println(awsErr.Code())
			// if "NoSuchBucket" == awsErr.Code() {
			// 	return *resp
			// }
		} else {
			fmt.Println(err.Error())
		}
		return
	}
}

type timer interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

type realClock struct{}

func (rc *realClock) Now() time.Time {
	return time.Now()
}

func (rc *realClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// ExcludeURL adds a new URL u to be ignored during logging. The URL u is parsed, hence the returned error
func (m *Middleware) ExcludeURL(u string) error {
	if _, err := url.Parse(u); err != nil {
		return err
	}
	m.excludeURLs[u] = struct{}{}
	return nil
}

// ExcludedURLs returns the list of excluded URLs for this middleware
func (m *Middleware) ExcludedURLs() []string {
	urls := []string{}
	for k, _ := range m.excludeURLs {
		urls = append(urls, k)
	}
	return urls
}

func (m *Middleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if m.Before == nil {
		m.Before = DefaultBefore
	}

	if m.After == nil {
		m.After = DefaultAfter
	}

	if _, ok := m.excludeURLs[r.URL.Path]; ok {
		return
	}

	start := m.clock.Now()

	// Try to get the real IP
	remoteAddr := r.RemoteAddr
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		remoteAddr = realIP
	}

	m.Before(m, r, remoteAddr)

	next(rw, r)

	latency := m.clock.Since(start)
	res := rw.(negroni.ResponseWriter)

	m.After(m, r, res, latency, remoteAddr)
}

// Called before handler
type BeforeFunc func(*Middleware, *http.Request, string)

// AfterFunc is the func type called after calling the next func in
// the middleware chain
type AfterFunc func(*Middleware, *http.Request, negroni.ResponseWriter, time.Duration, string)

// DefaultBefore is the default func assigned to *Middleware.Before
func DefaultBefore(m *Middleware, req *http.Request, remoteAddr string) {
	context.Set(req, PutMetric, m.PutMetric)
}

// DefaultAfter is the default func assigned to *Middleware.After
func DefaultAfter(m *Middleware, req *http.Request, res negroni.ResponseWriter, latency time.Duration, remoteAddr string) {
	ms := float64(latency.Nanoseconds() * 1000)
	m.PutMetric([]*cw.MetricDatum{
		{
			MetricName: aws.String(m.LatencyMetricName),
			Dimensions: []*cw.Dimension{
				{
					Name:  aws.String("RequestURI"),
					Value: aws.String(req.RequestURI),
				},
				{
					Name:  aws.String("RemoteAddr"),
					Value: aws.String(remoteAddr),
				},
			},
			Timestamp: aws.Time(time.Now()),
			Unit:      aws.String("Microseconds"),
			Value:     aws.Float64(ms),
		},
	})
	context.Clear(req)
}
