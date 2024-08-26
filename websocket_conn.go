package bitstamp

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	PingFrequency = time.Second * 10
	PongWait      = time.Second * 20
)

var (
	logger = logrus.WithField("service", "bitstamp").WithField("module", "wsconn")
)

// WSConn driver for receiving Order Book from exchanges working with WebSocket
type WSConn struct {
	conn     *websocket.Conn
	readerCh chan []byte
}

// NewWSConn creates a new *WebSocket instance
func NewWSConn(conn *websocket.Conn) *WSConn {
	return &WSConn{
		conn: conn,
	}
}

// keepalive sends ping messages to maintain websocket connection
func (ws *WSConn) keepalive(close chan struct{}) {
	// create a ticker to send ping messages every PingFrequency seconds
	tk := time.NewTicker(PingFrequency)
	defer tk.Stop()

	// if pong was sent in response, then the connection's deadline is updated
	ws.conn.SetPongHandler(func(appData string) error {
		logger.Debug("got pong message")
		if err := ws.conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
			logger.WithError(err).Error("could not update read-readline")
			return err
		}
		return nil
	})

	for {
		select {
		case <-close:
			return
		case <-tk.C:
			logger.Debug("sending ping")
			err := ws.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				logger.WithError(err).Error("could not send ping-message")
				return
			}
		}
	}
}

// reader reads data from WebSocket
func (ws *WSConn) reader(timeout time.Duration) {
	keepAliveStop := make(chan struct{})

	defer func() {
		close(keepAliveStop)
		close(ws.readerCh)
	}()

	go ws.keepalive(keepAliveStop)

	for {
		if err := ws.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			logger.WithError(err).Error("could not set deadline for websocket")
			return
		}
		_, msg, err := ws.conn.ReadMessage()
		if err != nil {
			logger.WithError(err).Error("could not read from websocket")
			return
		}

		ws.readerCh <- msg
	}
}

// RunReader starts the process of reading data from WebSocket
func (ws *WSConn) RunReader(wsTimeout time.Duration) <-chan []byte {
	ws.readerCh = make(chan []byte, 256)
	go ws.reader(wsTimeout)
	return ws.readerCh
}

// SendMessage sends a message via WebSocket protocol
func (ws *WSConn) SendMessage(msg string) error {
	return ws.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func (ws *WSConn) Stop() {
	_ = ws.conn.Close()
}
