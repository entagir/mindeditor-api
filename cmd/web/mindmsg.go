package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

func readMindMsg(conn *connection, connectionToken string, p []byte) {
	var msg clientMessage
	err := json.Unmarshal(p, &msg)
	if err != nil {
		log.Fatal(err)
		return
	}

	if msg.Action == "subscribe" {
		if file, ok := files[msg.FileId]; ok {
			file.subscribe(conn, connectionToken)
		} else {
			files[msg.FileId] = &mfile{
				id:   msg.FileId,
				subs: map[string]*connection{},
			}

			files[msg.FileId].subscribe(conn, connectionToken)
		}
		connections[connectionToken].files[msg.FileId] = files[msg.FileId]
		files[msg.FileId].broadcastUsers()
		//log.Println(files, files[msg.FileId], len(files[msg.FileId].subs))
	}

	if msg.Action == "unsubscribe" {
		if file, ok := files[msg.FileId]; ok {
			file.unsubscribe(connectionToken)
			if len(file.subs) == 0 {
				delete(files, msg.FileId)
			} else {
				file.broadcastUsers()
			}
		}
		delete(connections[connectionToken].files, msg.FileId)
		//log.Println(files, files[msg.FileId], len(files[msg.FileId].subs))
	}

	if msg.Action == "update" {
		//log.Println(msg)

		// TODO: check access

		var data interface{}
		if err := json.Unmarshal([]byte(msg.Data), &data); err != nil {
			log.Println(err)
			return
		}
		res := data.(map[string]interface{})

		eventType := ""
		if val, ok := res["type"]; ok {
			eventType = val.(string)
		}

		// TODO: validation
		if eventType == "" {
			return
		}

		ts := time.Now().UTC()

		eventId := ""
		if valId, ok := res["id"]; ok {
			eventId = valId.(string)
		}
		delete(res, "id")

		if file, ok := files[msg.FileId]; ok {
			msgDataBroadcast, _ := json.Marshal(res)
			msgb := eventMessage{
				Type:   "event",
				FileId: msg.FileId,
				Data:   string(msgDataBroadcast),
			}

			msgb.Timestamp = ts
			file.broadcast(msgb, connectionToken)
		}

		msgToSender := fmt.Sprint(`{"type": "event-done", "fileId": "`, msg.FileId,
			`", "eventId": "`, eventId, `", "timestamp": "`, ts.Format(time.RFC3339Nano), `"}`)
		sendMsg(conn.conn, []byte(msgToSender))

		delete(res, "type")

		if eventType == "file_remove" {
			// TODO: proc file_remove
		} else { // TODO: Skip interactive events
			fileName := ""
			if eventType == "file_rename" {
				name, _ := res["name"].(string)
				fileName = name
			}
			msgDataContent, _ := json.Marshal(res)
			createMindMapEvent(msg.FileId, eventType, string(msgDataContent), ts, &fileName)
		}
	}
}
