package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type EurekaClient struct {
	BaseURL          string
	HttpClient       *http.Client
	UAAClient        *UAAClient
	ServiceInstances []ServiceInstance
}

func (e *EurekaClient) RegisterAll() error {
	for _, s := range e.ServiceInstances {
		err := e.Register(s)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *EurekaClient) Register(serviceInstance ServiceInstance) error {
	token, err := e.UAAClient.GetToken()
	if err != nil {
		return err
	}

	postBody := map[string]interface{}{
		"instance": map[string]interface{}{
			"hostName": fmt.Sprintf("%s-%d-%d", serviceInstance.Name, serviceInstance.Instance, serviceInstance.Port),
			"app":      serviceInstance.Name,
			"ipAddr":   serviceInstance.IP,
			"status":   "UP",
			"port": map[string]interface{}{
				"$":        fmt.Sprintf("%d", serviceInstance.Port),
				"@enabled": "true",
			},
			"dataCenterInfo": map[string]interface{}{
				"@class": "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
				"name":   "MyOwn",
			},
		},
	}
	reqBytes, err := json.Marshal(postBody)
	if err != nil {
		return err
	}

	url, err := e.createURL(fmt.Sprintf("/eureka/apps/%s", serviceInstance.Name))
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", token))

	resp, err := e.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected response code: %d: %s", resp.StatusCode, respBytes)
	}

	return nil
}

func (e *EurekaClient) createURL(route string) (string, error) {
	u, err := url.Parse(e.BaseURL)
	if err != nil {
		return "", fmt.Errorf("unable to parse base url: %s", err)
	}
	u.Path = path.Join(u.Path, route)
	return u.String(), nil
}

type ServiceInstance struct {
	Name     string
	Instance int
	IP       string
	Port     int
}

type UAAClient struct {
	BaseURL string
	Name    string
	Secret  string
}

func (c *UAAClient) GetToken() (string, error) {
	bodyString := "grant_type=client_credentials"
	request, err := http.NewRequest("POST", c.BaseURL, strings.NewReader(bodyString))
	request.SetBasicAuth(c.Name, c.Secret)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	type getTokenResponse struct {
		AccessToken string `json:"access_token"`
	}
	response := &getTokenResponse{}
	err = c.makeRequest(request, response)
	if err != nil {
		return "", err
	}
	return response.AccessToken, nil
}

func (c *UAAClient) makeRequest(request *http.Request, response interface{}) error {
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("http client: %s", err)
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %s", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad uaa response, code %d, msg %s", resp.StatusCode, string(respBytes))
	}

	err = json.Unmarshal(respBytes, &response)
	if err != nil {
		return fmt.Errorf("unmarshal json: %s", err)
	}
	return nil
}
