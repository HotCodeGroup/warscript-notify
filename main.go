package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/HotCodeGroup/warscript-utils/balancer"
	"github.com/HotCodeGroup/warscript-utils/logging"
	"github.com/HotCodeGroup/warscript-utils/middlewares"
	"github.com/HotCodeGroup/warscript-utils/models"
	"google.golang.org/grpc"

	"github.com/sirupsen/logrus"

	consulapi "github.com/hashicorp/consul/api"
)

var authGPRC models.AuthClient
var logger *logrus.Logger

//nolint: gocyclo
func main() {
	h = &hub{
		sessions:   make(map[int64]map[string]chan *HubMessage),
		broadcast:  make(chan *HubMessage),
		register:   make(chan *HubClient),
		unregister: make(chan *HubClient),
	}
	go h.run()

	// коннекстим логер
	var err error
	logger, err = logging.NewLogger(os.Stdout, os.Getenv("LOGENTRIESRUS_TOKEN"))
	if err != nil {
		log.Printf("can not create logger: %s", err)
		return
	}

	// коннектим консул
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = os.Getenv("CONSUL_ADDR")
	consul, err := consulapi.NewClient(consulConfig)
	if err != nil {
		logger.Errorf("can not connect consul service: %s", err)
		return
	}

	httpPort, grpcPort, err := balancer.GetPorts("warscript-notify/bounds", "warscript-notify", consul)
	if err != nil {
		logger.Errorf("can not find empry port: %s", err)
		return
	}

	// коннектимся к серверу warscript-users по grpc
	authGPRCConn, err := balancer.ConnectClient(consul, "warscript-users-grpc")
	if err != nil {
		logger.Errorf("can not connect to auth grpc: %s", err.Error())
		return
	}
	defer authGPRCConn.Close()
	authGPRC = models.NewAuthClient(authGPRCConn)

	// регаем http сервис
	httpServiceID := fmt.Sprintf("warscript-notify-http:%d", httpPort)
	err = consul.Agent().ServiceRegister(&consulapi.AgentServiceRegistration{
		ID:      httpServiceID,
		Name:    "warscript-notify-http",
		Port:    httpPort,
		Address: "127.0.0.1",
	})
	if err != nil {
		logger.Errorf("can not register warscript-notify-http: %s", err.Error())
		return
	}
	defer func() {
		err = consul.Agent().ServiceDeregister(httpServiceID)
		if err != nil {
			logger.Errorf("can not derigister http service: %s", err)
		}
		logger.Info("successfully derigister http service")
	}()

	// регаем grpc сервис
	grpcServiceID := fmt.Sprintf("warscript-notify-grpc:%d", grpcPort)
	err = consul.Agent().ServiceRegister(&consulapi.AgentServiceRegistration{
		ID:      grpcServiceID,
		Name:    "warscript-notify-grpc",
		Port:    grpcPort,
		Address: "127.0.0.1",
	})
	if err != nil {
		logger.Errorf("can not register warscript-notify-grpc: %s", err.Error())
		return
	}
	defer func() {
		err = consul.Agent().ServiceDeregister(grpcServiceID)
		if err != nil {
			logger.Errorf("can not derigister grpc service: %s", err)
		}
		logger.Info("successfully derigister grpc service")
	}()

	// стартуем свой grpc
	notify := &NotifyManager{}
	listenGRPCPort, err := net.Listen("tcp", ":"+strconv.Itoa(grpcPort))
	if err != nil {
		logger.Errorf("grpc port listener error: %s", err)
		return
	}

	serverGRPCNotify := grpc.NewServer()

	models.RegisterNotifyServer(serverGRPCNotify, notify)
	logger.Infof("Notify gRPC service successfully started at port %d", grpcPort)
	go func() {
		if startErr := serverGRPCNotify.Serve(listenGRPCPort); startErr != nil {
			logger.Fatalf("Notify gRPC service failed at port %d: %v", grpcPort, startErr)
			os.Exit(1)
		}
	}()

	// стартуем http
	r := mux.NewRouter().PathPrefix("/v1").Subrouter()

	r.HandleFunc("/connect", middlewares.WithAuthentication(OpenWS, logger, authGPRC)).Methods("GET")
	http.Handle("/", middlewares.RecoverMiddleware(middlewares.AccessLogMiddleware(r, logger), logger))

	logger.Infof("Notify HTTP service successfully started at port %d", httpPort)
	err = http.ListenAndServe(":"+strconv.Itoa(httpPort), nil)
	if err != nil {
		logger.Errorf("cant start main server. err: %s", err.Error())
		return
	}
}
