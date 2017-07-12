package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pivotal-cf-experimental/go-eureka-example/lib"
	"github.com/ryanmoran/viron"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

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

	VCAPApplication struct {
		ApplicationName string `json:"application_name"`
		InstanceIndex   int    `json:"instance_index"`
	} `env:"VCAP_APPLICATION" env-required:"true"`

	CFInstanceInternalIP string `env:"CF_INSTANCE_INTERNAL_IP" env-required:"true"`
}

type InfoHandler struct {
	Port      int
	UserPorts string
}

var stylesheet template.HTML = template.HTML(`
<!-- Latest compiled and minified CSS -->
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap.min.css" integrity="sha384-1q8mTJOASx8j1Au+a5WDVnPi2lkFfwwEAa8hDDdjZlpLegxhjVME1fgjWPGmkzs7" crossorigin="anonymous">

<!-- Optional theme -->
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap-theme.min.css" integrity="sha384-fLW2N01lMqjakBkx3l/M9EahuwpSfeNvV63J5ezn3uZzapT0u7EYsXMjQV+0En5r" crossorigin="anonymous">

<!-- Latest compiled and minified JavaScript -->
<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/js/bootstrap.min.js" integrity="sha384-0mSbJDEHialfmuBBQP6A4Qrprq5OVfW37PRR3j5ELqxss1yVqOtnepnHVP9aJ7xS" crossorigin="anonymous"></script>
<style>
.jumbotron {
	text-align: center;
}
.header h3 {
	color: white;
}
</style>
`)

type PublicPage struct {
	Stylesheet    template.HTML
	OverlayIP     string
	InstanceIndex string
	UserPorts     string
}

var publicPageTemplate string = `
<!DOCTYPE html>
<html lang="en">
  <head>
	<title>Backend</title>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1">
	{{.Stylesheet}}
	</head>
	<body>
		<div class="container">
			<div class="header clearfix navbar navbar-inverse">
				<div class="container">
					<h3>Backend Sample App</h3>
				</div>
			</div>
			<div class="jumbotron">
				<h1>My overlay IP is: {{.OverlayIP}}</h1>
				<h3>My instance index is: {{.InstanceIndex}}</h3>
				<p class="lead">I'm serving cats on TCP ports {{.UserPorts}}</p>
			</div>
		</div>
	</body>
</html>
`

type CatPage struct {
	Stylesheet template.HTML
	Port       int
	IPAddr     string
}

var catPageTemplate string = `
<!DOCTYPE html>
<html lang="en">
	<head>
		<title>Backend</title>
		<meta charset="utf-8">
		<meta http-equiv="X-UA-Compatible" content="IE=edge">
		<meta name="viewport" content="width=device-width, initial-scale=1">
		{{.Stylesheet}}
	</head>
	<body>
		<div class="row">
			<div class="col-xs-8 col-xs-offset-2">
				<div class="header clearfix navbar navbar-inverse">
					<div class="container">
						<h3>Backend Sample App</h3>
					</div>
				</div>
				<div class="jumbotron">
					<p class="lead">Hello from the backend, here is a picture of a cat:</p>
					<p><img src="http://i.imgur.com/1uYroRF.gif" /></p>
				  <p class="lead">My IP is {{.IPAddr}}, you reached me on port {{.Port}}</p>
				</div>
			</div>
		</div>
	</body>
</html>
`

func (h *InfoHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	log.Printf("InfoHandler: request received from %s", req.RemoteAddr)
	instanceIndex := os.Getenv("CF_INSTANCE_INDEX")

	overlayIP := os.Getenv("CF_INSTANCE_INTERNAL_IP")
	template := template.Must(template.New("publicPage").Parse(publicPageTemplate))
	err := template.Execute(resp, PublicPage{
		Stylesheet:    stylesheet,
		OverlayIP:     overlayIP,
		InstanceIndex: instanceIndex,
		UserPorts:     h.UserPorts,
	})
	if err != nil {
		panic(err)
	}
	return
}

type CatHandler struct {
	Port   int
	IPAddr string
}

func (h *CatHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	log.Printf("CatHandler: request received from %s", req.RemoteAddr)
	template := template.Must(template.New("catPage").Parse(catPageTemplate))
	err := template.Execute(resp, CatPage{
		Stylesheet: stylesheet,
		Port:       h.Port,
		IPAddr:     h.IPAddr,
	})
	if err != nil {
		panic(err)
	}
	return
}

func launchCatHandler(port int) {
	mux := http.NewServeMux()
	mux.Handle("/", &CatHandler{
		Port: port,
	})
	httpServer := http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}
	httpServer.SetKeepAlivesEnabled(false)
	httpServer.ListenAndServe()
}

func generateReply(requestMessage []byte) []byte {
	return bytes.ToUpper(requestMessage)
}

func main() {
	var env Environment
	err := viron.Parse(&env)
	if err != nil {
		log.Fatalf("unable to parse environment: %s", err)
	}

	systemPortString := os.Getenv("PORT")
	systemPort, err := strconv.Atoi(systemPortString)
	log.SetOutput(os.Stdout)
	if err != nil {
		log.Fatal("invalid required env var PORT")
	}

	userPorts, err := extractPortNumbers("CATS_PORTS")
	if err != nil {
		log.Fatal(err.Error())
	}

	localIP := env.CFInstanceInternalIP

	infoHandler := &InfoHandler{
		Port:      systemPort,
		UserPorts: os.Getenv("CATS_PORTS"),
	}

	server := http_server.New(fmt.Sprintf("0.0.0.0:%d", systemPort), infoHandler)

	members := []grouper.Member{grouper.Member{"info_server", server}}

	var serviceInstances []lib.ServiceInstance
	for _, userPort := range userPorts {
		serviceInstances = append(serviceInstances, lib.ServiceInstance{
			Name:     env.VCAPApplication.ApplicationName,
			Instance: env.VCAPApplication.InstanceIndex,
			IP:       localIP,
			Port:     userPort,
		})
		catHandler := &CatHandler{
			Port:   userPort,
			IPAddr: localIP,
		}
		members = append(members, grouper.Member{fmt.Sprintf("cat_server_%d", userPort),
			http_server.New(fmt.Sprintf("0.0.0.0:%d", userPort), catHandler)})
	}

	serviceCredentials := env.VCAPServices.ServiceRegistry[0].Credentials

	uaaClient := &lib.UAAClient{
		BaseURL: serviceCredentials.AccessTokenURI,
		Name:    serviceCredentials.ClientID,
		Secret:  serviceCredentials.ClientSecret,
	}

	eurekaClient := &lib.EurekaClient{
		BaseURL:          serviceCredentials.RegistryURI,
		HttpClient:       http.DefaultClient,
		UAAClient:        uaaClient,
		ServiceInstances: serviceInstances,
	}

	pollInterval := 20 * time.Second // we can fail twice and not lose presence in the registry
	poller := &Poller{
		PollInterval: pollInterval,
		Action:       eurekaClient.RegisterAll,
	}

	// poller goes at the end, so that registration happens after all servers start
	members = append(members, grouper.Member{"registration_poller", poller})

	monitor := ifrit.Invoke(sigmon.New(grouper.NewOrdered(os.Interrupt, members)))

	err = <-monitor.Wait()
	if err != nil {
		log.Fatalf("ifrit monitor: %s", err)
	}

}

func extractPortNumbers(envVarName string) ([]int, error) {
	portStrings := strings.Split(os.Getenv(envVarName), ",")
	portNumbers := []int{}
	for _, portString := range portStrings {
		if strings.TrimSpace(portString) == "" {
			continue
		}
		portNumber, err := strconv.Atoi(portString)
		if err != nil {
			return nil, fmt.Errorf("invalid port %s", portString)
		}

		portNumbers = append(portNumbers, portNumber)
	}
	return portNumbers, nil
}

type Poller struct {
	PollInterval time.Duration
	Action       func() error
}

func (m *Poller) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := m.Action()
	if err != nil {
		return err
	}

	close(ready)

	for {
		select {
		case <-signals:
			return nil
		case <-time.After(m.PollInterval):
			err = m.Action()
			if err != nil {
				log.Printf("%s", err)
				continue
			}
		}
	}
}
