# negroni-cloudwatch

[![GoDoc](https://godoc.org/github.com/cvillecsteele/negroni-cloudwatch?status.svg)](https://godoc.org/github.com/cvillecsteele/negroni-cloudwatch)
[![Build Status](https://travis-ci.org/cvillecsteele/negroni-cloudwatch.svg?branch=master)](https://travis-ci.org/cvillecsteele/negroni-cloudwatch)

AWS cloudwatch middleware for negroni.

## Installation

```shell
go get -u github.com/cvillecsteele/negroni-cloudwatch
```

## Usage

    package main

    import (
            "fmt"
            "net/http"

            "github.com/aws/aws-sdk-go/aws"
            cw "github.com/aws/aws-sdk-go/service/cloudwatch"

            "github.com/gorilla/context"
            "github.com/codegangsta/negroni"
            ncw "github.com/cvillecsteele/negroni-cloudwatch"
    )

    func main() {
            r := http.NewServeMux()
            r.HandleFunc(`/`, func(w http.ResponseWriter, r *http.Request) {
                    put := context.Get(req, ncw.PutMetric)
                    put([]*cw.MetricDatum{
                        {
                                MetricName: aws.String("MyMetric"),
                                Dimensions: []*cw.Dimension{
                                        {
                                                Name:  aws.String("ThingOne"),
                                                Value: aws.String("something"),
                                        },
                                        {
                                                Name:  aws.String("ThingTwo"),
                                                Value: aws.String("other"),
                                        },
                                },
                                Timestamp: aws.Time(time.Now()),
                                Unit:      aws.String("Count"),
                                Value:     aws.Float64(42),
                        },
                    })
                    w.WriteHeader(http.StatusOK)
                    fmt.Fprintf(w, "success!\n")
            })

            n := negroni.New()
            n.Use(ncw.New("us-east-1", "test"))
            n.UseHandler(r)

            n.Run(":9999")
    }

