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

var upgrader = websocket.Upgrader{}

var devicesWs = make(map[int]*websocket.Conn)
var devicesWsMutex = &sync.RWMutex{}
var devices = []Device{}
var devicesMutex = &sync.RWMutex{}

func setDeviceValue(id int, value int) {

	fmt.Printf("Setting value of device %d to %d", id, value)

	ws := devicesWs[id]
	msg, _ := json.Marshal(map[string]int{"value": value})
	ws.WriteMessage(websocket.TextMessage, msg)
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
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			fmt.Println(err)
			return
		}

		defer ws.Close()

	loop:
		for {
			mt, message, err := ws.ReadMessage()
			if err != nil {
				fmt.Println(err)
				break
			}

			switch mt {
			case websocket.TextMessage:
				var msg Device
				err = json.Unmarshal(message, &msg)
				if err != nil {
					fmt.Println(err)
					break loop
				}

				fmt.Printf("Connection received from device %v", msg)

				devicesWsMutex.Lock()
				devicesWs[msg.Id] = ws
				devicesWsMutex.Unlock()

				devicesMutex.Lock()
				devices = append(devices, msg)
				devicesMutex.Unlock()

			default:
				fmt.Printf("Unhandled message type %d", mt)
				break loop
			}
		}

	})

	r.Run(":8080")
}
