package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Device struct {
	Id          int    `json:"id"`
	Description string `json:"description"`
	Value       int    `json:"value"`
}

type DeviceValue struct {
	Id    int `json:"id"`
	Value int `json:"value"`
}

type DeviceWs struct {
	device *Device
	ws     *websocket.Conn
}

type Message struct {
	Id      string      `json:"id"`
	Payload interface{} `json:"payload"`
}

type GenericMessage struct {
	Id      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

var upgrader = websocket.Upgrader{}

var devicesWs = make(map[int]DeviceWs)
var devicesWsMutex = &sync.RWMutex{}

var uis = make(map[*websocket.Conn]struct{})
var uisMutex = &sync.RWMutex{}

func setDeviceValue(id int, value int) {

	fmt.Printf("Setting value of device %d to %d", id, value)

	devicesWsMutex.Lock()
	dws := devicesWs[id]
	dws.device.Value = value
	devicesWsMutex.Unlock()

	msg, _ := json.Marshal(map[string]int{"value": value})
	dws.ws.WriteMessage(websocket.TextMessage, msg)
}

func sendToUis(msg Message) {

	uisMutex.RLock()

	for ws := range uis {
		b, _ := json.Marshal(msg)
		ws.WriteMessage(websocket.TextMessage, b)
	}

	uisMutex.RUnlock()
}

func handleDeviceConnect(device Device) {
	sendToUis(Message{"add", device})
}

func handleUiMessage(msg Message) {
	switch t := msg.Payload.(type) {
	case DeviceValue:
		setDeviceValue(t.Id, t.Value)
		sendToUis(Message{Id: "update", Payload: msg.Payload})
	default:
		fmt.Printf("Unhandled message: %s %v\n", reflect.TypeOf(msg.Payload), msg)
	}

}

func main() {
	r := gin.Default()

	r.GET("/rest/device/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		value, _ := strconv.Atoi(c.Query("value"))
		setDeviceValue(id, value)
		c.String(http.StatusOK, "")

	})

	r.GET("/ws/devices", func(c *gin.Context) {

		fmt.Printf("/ws/devices : incoming connection from %s\n", c.ClientIP())

		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			fmt.Println(err)
			return
		}

		defer ws.Close()

		var msg Device

		defer func() {
			devicesWsMutex.Lock()
			delete(devicesWs, msg.Id)
			devicesWsMutex.Unlock()
		}()

	loop:
		for {
			mt, message, err := ws.ReadMessage()
			if err != nil {
				switch err.(type) {
				case *websocket.CloseError:
					if msg.Id != 0 {
						fmt.Printf("Devices %v disconnected\n", msg.Id)
					} else {
						fmt.Printf("Client %v disconnected\n", c.ClientIP())
					}
				default:
					fmt.Println(err)
				}
				break
			}

			switch mt {
			case websocket.TextMessage:
				err = json.Unmarshal(message, &msg)
				if err != nil {
					fmt.Println(err)
					break loop
				}

				fmt.Printf("Device %v connected\n", msg)

				devicesWsMutex.Lock()
				devicesWs[msg.Id] = DeviceWs{&msg, ws}
				devicesWsMutex.Unlock()

				handleDeviceConnect(msg)

			default:
				fmt.Printf("Unhandled message type %d\n", mt)
				break loop
			}
		}

	})

	r.GET("/ws/ui", func(c *gin.Context) {
		fmt.Printf("/ws/ui : incoming connection from %s\n", c.ClientIP())

		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			fmt.Println(err)
			return
		}

		defer ws.Close()
		defer func() {
			delete(uis, ws)
		}()
		uisMutex.Lock()
		uis[ws] = struct{}{}
		uisMutex.Unlock()

		devicesWsMutex.RLock()
		devices := make([]Device, 0, len(devicesWs))
		for _, v := range devicesWs {
			devices = append(devices, *v.device)
		}
		devicesWsMutex.RUnlock()

		msg, err := json.Marshal(Message{"init", devices})
		if err != nil {
			fmt.Println(err)
			return
		}
		ws.WriteMessage(websocket.TextMessage, msg)

	loop:
		for {
			mt, message, err := ws.ReadMessage()
			if err != nil {
				fmt.Println(err)
				break
			}

			switch mt {
			case websocket.TextMessage:
				var msg GenericMessage
				json.Unmarshal(message, &msg)

				switch msg.Id {
				case "value":
					var v DeviceValue
					json.Unmarshal(msg.Payload, &v)
					handleUiMessage(Message{msg.Id, v})
				}

			default:
				fmt.Printf("Unhandled message type %d\n", mt)
				break loop
			}

		}
	})

	r.Run(":8080")
}
