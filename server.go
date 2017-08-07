package msgbus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"golang.org/x/net/websocket"

	"github.com/julienschmidt/httprouter"
)

// Server ...
type Server struct {
	bus    *MessageBus
	router *httprouter.Router
}

// IndexHandler ...
func (s *Server) IndexHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		fmt.Fprint(w, "Welcome!\n")
	}
}

// PushHandler ...
func (s *Server) PushHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		topic := p.ByName("topic")
		websocket.Handler(s.PushWebSocketHandler(topic)).ServeHTTP(w, r)
	}
}

// PullHandler ...
func (s *Server) PullHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		topic := p.ByName("topic")
		message, ok := s.bus.Get(topic)
		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		out, err := json.Marshal(message)
		if err != nil {
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			return
		}

		w.Write(out)
	}
}

// PutHandler ...
func (s *Server) PutHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		topic := p.ByName("topic")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			return
		}

		s.bus.Put(topic, s.bus.NewMessage(body))
	}
}

// PushWebSocketHandler ...
func (s *Server) PushWebSocketHandler(topic string) websocket.Handler {
	return func(conn *websocket.Conn) {
		id := conn.Request().RemoteAddr
		ch := s.bus.Subscribe(id, topic)
		defer func() {
			s.bus.Unsubscribe(id, topic)
		}()

		var err error

		for {
			msg := <-ch
			err = websocket.JSON.Send(conn, msg)
			if err != nil {
				log.Printf("Error sending msg to %s", id)
				continue
			}
		}
	}
}

func (s *Server) ListenAndServe(bind string) error {
	return http.ListenAndServe(bind, s.router)
}

func (s *Server) initRoutes() {
	s.router.GET("/", s.IndexHandler())
	s.router.GET("/push/:topic", s.PushHandler())
	s.router.GET("/pull/:topic", s.PullHandler())
	s.router.PUT("/:topic", s.PutHandler())
}

// NewServer ...
func NewServer(bus *MessageBus) *Server {
	if bus == nil {
		bus = NewMessageBus()
	}

	server := &Server{
		bus:    bus,
		router: httprouter.New(),
	}

	server.initRoutes()

	return server
}
