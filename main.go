package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/go-redis/redis"

	"github.com/gorilla/mux"

	"github.com/HotCodeGroup/warscript-utils/balancer"
	"github.com/HotCodeGroup/warscript-utils/logging"
	"github.com/HotCodeGroup/warscript-utils/middlewares"
	"github.com/HotCodeGroup/warscript-utils/models"
	redisUtils "github.com/HotCodeGroup/warscript-utils/redis"
	"google.golang.org/grpc"

	"github.com/sirupsen/logrus"

	vk "github.com/GDVFox/vkbot-go"
	consulapi "github.com/hashicorp/consul/api"
	vaultapi "github.com/hashicorp/vault/api"
)

var rediCli *redis.Client
var notifyVKBot vk.VkBot
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

	// коннектим волт
	vaultConfig := vaultapi.DefaultConfig()
	vaultConfig.Address = os.Getenv("VAULT_ADDR")
	vault, err := vaultapi.NewClient(vaultConfig)
	if err != nil {
		logger.Errorf("can not connect vault service: %s", err)
		return
	}
	vault.SetToken(os.Getenv("VAULT_TOKEN"))

	vkConf, err := vault.Logical().Read("warscript-notify/vk")
	if err != nil || vkConf == nil || len(vkConf.Warnings) != 0 {
		logger.Errorf("can read warscript-games/postges key: %+v; %+v", err, vkConf)
		return
	}

	groupID, err := strconv.ParseInt(vkConf.Data["group_id"].(string), 10, 64)
	if err != nil {
		logger.Errorf("can not parse vk bot group ID: %s", err)
		return
	}

	redisConf, err := vault.Logical().Read("warscript-notify/redis")
	if err != nil || redisConf == nil || len(redisConf.Warnings) != 0 {
		logger.Errorf("can read config/redis key: %s; %+v", err, redisConf.Warnings)
		return
	}

	notifyVKBot, err := vk.NewVkBot(vkConf.Data["token"].(string), vkConf.Data["version"].(string), &http.Client{})
	if err != nil {
		logger.Errorf("can not create vk bot: %s", err)
		return
	}
	notifyVKBot.SetConfirmation(
		vk.NewConfirmation("/vk", groupID, vkConf.Data["confirmation"].(string)))
	events := notifyVKBot.ListenForEvents()
	go ProcessVKEvents(events)

	rediCli, err = redisUtils.Connect(redisConf.Data["user"].(string),
		redisConf.Data["pass"].(string), redisConf.Data["addr"].(string),
		redisConf.Data["database"].(string))
	if err != nil {
		logger.Errorf("can not connect redis: %s", err)
		return
	}
	defer rediCli.Close()

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
