package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/pivotal-cf-experimental/go-eureka-example/lib"
	"github.com/ryanmoran/viron"
)

type HttpDemoResultPage struct {
	Stylesheet template.HTML
	CatBody    template.HTML
}

var httpDemoResultPageTemplate string = `
<!DOCTYPE html>
<html lang="en">
	<head>
		<title>Frontend</title>
		<meta charset="utf-8">
		<meta http-equiv="X-UA-Compatible" content="IE=edge">
		<meta name="viewport" content="width=device-width, initial-scale=1">
		{{.Stylesheet}}
	</head>
	<body>
		<div class="container">
			<div class="header clearfix navbar navbar-inverse">
				<div class="container">
					<h3>Frontend Sample App</h3>
				</div>
			</div>

			{{.CatBody}}
		</div>
	</body>
</html>
`

type Environment struct {
	VCAPServices struct {
		ServiceRegistry []struct {
			Credentials struct {
				RegistryURI    string `json:"uri"`
				ClientSecret   string `json:"client_secret"`
				ClientID       string `json:"client_id"`
				AccessTokenURI string `json:"access_token_uri"`
			} `json:"credentials"`
		} `json:"p-service-registry"`
	} `env:"VCAP_SERVICES" env-required:"true"`
}

type HttpDemoHandler struct{}

func (h *HttpDemoHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	queryParams, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		panic(err)
	}
	appName := queryParams["app"][0]

	var env Environment
	err = viron.Parse(&env)
	if err != nil {
		log.Fatalf("unable to parse environment: %s", err)
	}
	serviceCredentials := env.VCAPServices.ServiceRegistry[0].Credentials

	uaaClient := &lib.UAAClient{
		BaseURL: serviceCredentials.AccessTokenURI,
		Name:    serviceCredentials.ClientID,
		Secret:  serviceCredentials.ClientSecret,
	}

	eurekaClient := &lib.EurekaClient{
		BaseURL:    serviceCredentials.RegistryURI,
		HttpClient: http.DefaultClient,
		UAAClient:  uaaClient,
	}

	destination, err := eurekaClient.GetAppByName(appName)
	if err != nil {
		panic(err)
	}

	destination = "http://" + destination
	httpClient := http.DefaultClient
	httpClient.Timeout = 5 * time.Second
	getResp, err := httpClient.Get(destination)
	if err != nil {
		template := template.Must(template.New("errorPageTemplate").Parse(errorPageTemplate))
		err = template.Execute(resp, ErrorPage{
			Stylesheet: stylesheet,
			Error:      err,
		})
		if err != nil {
			panic(err)
		}

		return
	}
	defer getResp.Body.Close()

	readBytes, err := ioutil.ReadAll(getResp.Body)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(fmt.Sprintf("read body failed: %s", err)))
		return
	}

	theTemplate := template.Must(template.New("httpDemoResultPage").Parse(httpDemoResultPageTemplate))
	catBody := template.HTML(string(readBytes))
	err = theTemplate.Execute(resp, HttpDemoResultPage{
		Stylesheet: stylesheet,
		CatBody:    catBody,
	})
	if err != nil {
		panic(err)
	}
}
