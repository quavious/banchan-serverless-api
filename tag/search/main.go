package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/projects/banchan-app/models"
	"github.com/projects/banchan-app/pkg"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse
type Request events.APIGatewayProxyRequest

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, request Request) (Response, error) {
	var buf bytes.Buffer

	conn := new(pkg.Conn)
	dbCtx, db := conn.GetConnection()
	defer dbCtx.Done()

	if db == nil {
		return Response{StatusCode: 501, Body: `{"message": "DB Connection Error"}`}, errors.New("DB Connection Error")
	}

	defer db.Close()

	_keyword := request.PathParameters["keyword"]
	_keywords, err := url.PathUnescape(_keyword)
	if err != nil {
		log.Println(err)
		return Response{StatusCode: 501, Body: `{"message": "Parameter Error"}`}, err
	}

	pathID := request.PathParameters["id"]
	id, err := strconv.Atoi(pathID)
	if err != nil || id < 1 {
		log.Println(err)
		return Response{StatusCode: 501, Body: `{"message": "Parameter Error"}`}, err
	}

	keywords := new(pkg.StringArray)
	*keywords = strings.Split(_keywords, " ")
	keywords = keywords.Filter(func(t string) bool {
		if len(t) < 1 {
			return false
		}
		return true
	})

	keywordList := []qm.QueryMod{}
	keywordList = append(keywordList, qm.OrderBy("id asc"))

	for i, item := range *keywords {
		var keyword qm.QueryMod
		if i == 0 {
			keyword = qm.Where("name like ?", "%"+item+"%")
		} else {
			keyword = qm.Or("name like ?", "%"+item+"%")
		}
		keywordList = append(keywordList, keyword)
	}
	keywordList = append(keywordList, qm.Limit(20))
	keywordList = append(keywordList, qm.Offset((id-1)*20))

	tags, err := models.Tags(
		keywordList...,
	).All(dbCtx, db)

	if err != nil {
		log.Println(err)
		return Response{StatusCode: 501, Body: `{"message": "DB Connection Error"}`}, err
	}

	body, err := json.Marshal(map[string]interface{}{
		"message": "OK",
		"tags":    tags,
	})
	if err != nil {
		log.Println(err)
		return Response{StatusCode: 404}, err
	}
	json.HTMLEscape(&buf, body)

	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers: map[string]string{
			"Content-Type":                     "application/json",
			"Access-Control-Allow-Origin":      "*",
			"Access-Control-Allow-Credentials": "true",
			"Access-Control-Allow-Methods":     "GET",
		},
	}

	return resp, nil
}

func main() {
	lambda.Start(Handler)
}
