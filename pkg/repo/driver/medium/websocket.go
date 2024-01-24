package medium

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	uuidLib "github.com/google/uuid"
	"github.com/gorilla/websocket"

	"notiboy/utilities"
)

type ErrWSConnAbsent struct {
	Message string
	ID      string
}

func (e *ErrWSConnAbsent) Error() string {
	return fmt.Sprintf("%s, ID: %s", e.Message, e.ID)
}

type Socket struct {
	*sync.RWMutex
	ConnSet     map[string]*UserConnObject
	ReadChannel chan Message
	WithReader  bool
}

type UserConnObject struct {
	ConnObjs    []*ConnObject
	IsOnline    bool
	LastChecked time.Time
}

type ConnObject struct {
	ID    string
	Conn  *websocket.Conn
	Close chan bool
}

type Message struct {
	Data     string `json:"data"`
	Receiver string `json:"receiver"`
	Chain    string `json:"chain"`
	Sender   string `json:"sender"`
	Time     int64  `json:"time"`
	Kind     string `json:"kind"`
	UUID     string `json:"uuid"`
}

const (
	pingInterval = time.Second * 30
)

func (s *Socket) GetReadChannel() chan Message {
	if !s.WithReader {
		return nil
	}
	return s.ReadChannel
}

func FormatIdentifier(chain, address string) string {
	return chain + "_" + address
}

func (s *Socket) Add(chain, address string, newUserConn *websocket.Conn) {
	s.Lock()
	defer s.Unlock()
	id := FormatIdentifier(chain, address)
	log := utilities.NewLoggerWithFields(
		"websocket.Add", map[string]interface{}{
			"id": id,
		},
	)

	if _, ok := s.ConnSet[id]; !ok {
		s.ConnSet[id] = &UserConnObject{
			ConnObjs: make([]*ConnObject, 0),
		}
	}

	connObj := &ConnObject{
		Conn:  newUserConn,
		Close: make(chan bool),
		ID:    uuidLib.NewString(),
	}

	err := connObj.Conn.SetWriteDeadline(time.Time{})
	if err != nil {
		log.WithError(err).Errorf("setting SetWriteDeadline failed for id %s", id)
	}

	connObj.Conn.SetCloseHandler(
		func(code int, text string) error {
			close(connObj.Close)
			log.Infof("Received close message with code %d and text %s for id %s:%s", code, text, id, connObj.ID)
			return nil
		},
	)

	readerFn := func(connObj *ConnObject, id string) {
		defer close(connObj.Close)
		thisConn := connObj.Conn
		for {
			log.Debugf("Waiting for message, id: %s", id)
			select {
			default:
				messageType, message, err := thisConn.ReadMessage()
				if err != nil {
					closeErr := &websocket.CloseError{}
					if !errors.As(err, &closeErr) {
						log.WithError(err).Errorf("error reading msg of type %d", messageType)
					}
					return
				}
				_ = thisConn.SetReadDeadline(time.Now().Add(pingInterval))
				var msg Message
				err = json.Unmarshal(message, &msg)
				if err != nil {
					log.WithError(err).Errorf("failed to unmarshal message %v", string(message))
				} else {
					if msg.Sender == "" {
						msg.Sender = address
					}
					if msg.Time == 0 {
						msg.Time = utilities.UnixTime()
					}
					s.ReadChannel <- msg
				}
			}
		}
	}

	if s.WithReader {
		go readerFn(connObj, id)
	}

	// to check health of connection
	go func(s *Socket, connObj *ConnObject, id string) {
		thisConn := connObj.Conn
		ticker := time.NewTicker(pingInterval)
		defer func() {
			log.Infof("Closing the ws connection for %s:%s", id, connObj.ID)
			ticker.Stop()
			err = thisConn.WriteMessage(
				websocket.CloseMessage, websocket.FormatCloseMessage(
					websocket.
						CloseNormalClosure, "",
				),
			)
			if err != nil {
				log.WithError(err).Error("sending close msg failed")
			}
			s.Remove(id, connObj.ID)
		}()

		_ = thisConn.SetReadDeadline(time.Now().Add(pingInterval))
		thisConn.SetPongHandler(func(string) error { _ = thisConn.SetReadDeadline(time.Now().Add(pingInterval)); return nil })

		for {
			if err = thisConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.WithError(err).Errorf("ping failed, id: %s", id)
				return
			}

			s.ConnSet[id].IsOnline = true
			s.ConnSet[id].LastChecked = time.Now()

			select {
			case <-connObj.Close:
				log.Debugf("Received ping close for %s", id)
				return
			case <-ticker.C:
			}
		}
	}(s, connObj, id)

	s.ConnSet[id].ConnObjs = append(
		s.ConnSet[id].ConnObjs, connObj,
	)
	log.Debugf("Adding new ws connection %s for user %s, total conns: %d", connObj.ID, id, len(s.ConnSet[id].ConnObjs))
}

func (s *Socket) Remove(identifier string, connID string) {
	log := utilities.NewLoggerWithFields(
		"websocket.Remove", map[string]interface{}{
			"id": identifier,
		},
	)

	s.Lock()
	defer s.Unlock()
	userConnObj, ok := s.ConnSet[identifier]
	if !ok || userConnObj == nil {
		// nothing to remove
		return
	}

	acceptedConns := make([]*ConnObject, 0)
	for _, connObj := range userConnObj.ConnObjs {
		if connObj.ID == connID {
			err := connObj.Conn.Close()
			if err != nil {
				log.WithError(err).Errorf("error closing ws conn for id %s", identifier)
			}
			continue
		}
		acceptedConns = append(acceptedConns, connObj)
	}

	if len(acceptedConns) == 0 {
		delete(s.ConnSet, identifier)
	} else {
		s.ConnSet[identifier].ConnObjs = acceptedConns
	}
}

func NewWebSocket(withReader bool) *Socket {
	socketConnect := &Socket{
		RWMutex:     new(sync.RWMutex),
		ConnSet:     make(map[string]*UserConnObject),
		ReadChannel: make(chan Message, 1000),
		WithReader:  withReader,
	}

	return socketConnect
}

func Upgrade() websocket.Upgrader {
	return websocket.Upgrader{
		Subprotocols: []string{"websocket"},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}

func (s *Socket) PushMessage(identifier string, data []byte, broadcast bool) error {
	s.RLock()
	defer s.RUnlock()
	log := utilities.NewLoggerWithFields(
		"websocket.PushMessage", map[string]interface{}{
			"id": identifier,
		},
	)

	userConnObj, ok := s.ConnSet[identifier]
	if !ok || userConnObj == nil || len(userConnObj.ConnObjs) < 1 {
		return &ErrWSConnAbsent{
			Message: "ws connection absent",
			ID:      identifier,
		}
	}

	connObjs := userConnObj.ConnObjs
	if !broadcast {
		connObjs = []*ConnObject{userConnObj.ConnObjs[len(userConnObj.ConnObjs)-1]}
	}

	sent := false
	var pushErrors []string
	for _, connObj := range connObjs {
		// Send the message over the WebSocket connection
		err := connObj.Conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			pushErrors = append(pushErrors, err.Error())
			//_ = connObj.Conn.Close()
			continue
		}
		sent = true
		log.Debugf("ws message %s sent to %s", string(data), identifier)
	}

	if !sent {
		return fmt.Errorf("ws message %s failed for %s: %s", string(data), identifier, strings.Join(pushErrors, ":"))
	}

	return nil
}
