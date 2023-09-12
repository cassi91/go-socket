package main

import (
	"net"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	logFile := &lumberjack.Logger{
		Filename:   "app.log",
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     7, // days
		Compress:   true,
	}

	logger := logrus.New()
	logger.Out = logFile

	listenAddress := "0.0.0.0:8860"
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		logger.Fatal("Error listening: ", err)
	}
	defer listener.Close()

	logger.Info("Listening on: ", listenAddress)

	//创建MQTT客户端
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://123.206.94.11:1883")

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Error("MQTT Connection Error: ", token.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Error accepting connection: ", err)
			continue
		}

		go handleConnection(conn, logger, client)
	}
}

func handleConnection(conn net.Conn, logger *logrus.Logger, mqttClient mqtt.Client) {
	defer conn.Close()

	logger.Info("Accepted connection from: ", conn.RemoteAddr())

	buffer := make([]byte, 1024)
	var receivedData strings.Builder

	for {
		bytesRead, err := conn.Read(buffer)
		if err != nil {
			if err.Error() == "EOF" {
				logger.Info("Connection closed by remote host: ", conn.RemoteAddr())
				break
			}
			logger.Error("Error reading: ", err)
			break
		}

		receivedData.Write(buffer[:bytesRead])

	}

	// 将接收到的数据写入日志
	logger.WithField("ReceivedData", receivedData.String()).Info("Received data from", conn.RemoteAddr())
	if token := mqttClient.Publish("smartpower/test", 0, false, receivedData.String()); token.Wait() && token.Error() != nil {
		logger.Error("Error publish Message: ", token.Error())
	}
}
