package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var connections = make(map[string]net.Conn)

func main() {
	logFile := &lumberjack.Logger{
		Filename:   "app.log",
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     7, // days
		Compress:   true,
	}

	// var mu sync.Mutex
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger := logrus.New()
	logger.Out = multiWriter

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
	opts.SetClientID("cassi1991")

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Error("MQTT Connection Error: ", token.Error())
	}

	//订阅command Topic, 用于处理接口命令
	if token := client.Subscribe("/smartpower/command", 0, func(c mqtt.Client, m mqtt.Message) {
		handleCommand(c, m, logger)
	}); token.Wait() && token.Error() != nil {
		logger.Error("MQTT Subscribe Error: ", token.Error())
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

	// 用 **
	dataList := strings.Split(receivedData.String(), "**")
	for _, value := range dataList {
		value = strings.Replace(value, " ", "", -1)
		logger.Info("Data", value)
		var r map[string]string
		if len(value) > 0 {
			if err := json.Unmarshal([]byte(value), &r); err != nil {
				logger.WithField("Data: ", value).Error("ReceivedData decode error: ", err)
				return
			}
			device_mac := r["DeviceID"]
			connections[device_mac] = conn
			message := strings.ToLower(r["Message"])

			var mqtt_topic string
			switch message {
			case "status":
				mqtt_topic = "/smartpower/status"
			case "systeminfo":
				mqtt_topic = "/smartpower/systeminfo"
			default:
				mqtt_topic = "/smartpower/settingres"
			}
			logger.Info("Send to Topic: ", mqtt_topic)
			var res_str string
			res_str = handleResponse(r, 1)
			if token := mqttClient.Publish(mqtt_topic, 0, false, value); token.Wait() && token.Error() != nil {
				logger.Error("Error publish Message: ", token.Error())
				res_str = handleResponse(r, 0)
			}
			conn.Write([]byte(res_str))
		}
	}
}

func handleCommand(mqttClient mqtt.Client, mqttMessage mqtt.Message, logger *logrus.Logger) {
	message := string(mqttMessage.Payload())
	m := strings.Replace(message, "**", "", -1)
	logger.Info("Received Message: ", message)
	var r map[string]string
	if err := json.Unmarshal([]byte(m), &r); err != nil {
		logger.WithField("Data: ", message).Error("ReceivedData decode error: ", err)
		return
	}
	device_mac := r["DeviceID"]
	conn := connections[device_mac]
	conn.Write(mqttMessage.Payload())
}

func handleResponse(p map[string]string, f int) string {
	var result string
	msg := p["Message"]

	if f == 1 {
		result = "1"
	} else {
		result = "0"
	}

	switch msg {
	case "STATUS":
		return fmt.Sprintf("{\"Message\": \"RESP STATUS\", \"DeviceID\": \"%s\", \"Result\": \"%s\"}", p["DeviceID"], result)
	case "SYSTEM INFO":
		return fmt.Sprintf("{\"Message\": \"RESP SYSTEM INFO\", \"DeviceID\": \"%s\", \"Result\": \"%s\"}", p["DeviceID"], result)
	default:
		return fmt.Sprintf("{\"Message\": \"%s\", \"DeviceID\": \"%s\", \"Result\": \"%s\"}", p["Message"], p["DeviceID"], result)
	}
}
