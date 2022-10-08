package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type DeviceWs struct {
	device Device
	ws     *websocket.Conn
}

var upgrader = websocket.Upgrader{}

var devicesWs = make(map[int]DeviceWs)
var devicesWsMutex = &sync.RWMutex{}

func setDeviceValue(id int, value int) {

	fmt.Printf("Setting value of device %d to %d", id, value)

	devicesWsMutex.RLock()
	dws := devicesWs[id]
	devicesWsMutex.RUnlock()

	msg, _ := json.Marshal(map[string]int{"value": value})
	dws.ws.WriteMessage(websocket.TextMessage, msg)
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

		fmt.Printf("Incoming connection from %s\n", c.ClientIP())

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
				devicesWs[msg.Id] = DeviceWs{msg, ws}
				devicesWsMutex.Unlock()

			default:
				fmt.Printf("Unhandled message type %d\n", mt)
				break loop
			}
		}

	})

	r.Run(":8080")
}
