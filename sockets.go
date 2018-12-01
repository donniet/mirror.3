package main

import (
  "sync"
  server "github.com/donniet/mirror.3/serveJSON"
  "github.com/gorilla/websocket"
  "encoding/base64"
  "crypto/rand"
  "log"
  "net/http"
  "encoding/json"
)

type Sockets struct {
  connections map[string]*websocket.Conn
  upgrader websocket.Upgrader
  lock sync.Locker
  Incoming chan *server.Request
}

type Connection struct {
  Conn *websocket.Conn
  Name string
}


func NewSockets() *Sockets {
  s := &Sockets{
    connections: make(map[string]*websocket.Conn),
    upgrader: websocket.Upgrader{
      ReadBufferSize: 1024,
      WriteBufferSize: 1024,
    },
    lock: &sync.Mutex{},
    Incoming: make(chan *server.Request),
  }
  return s
}
func (s *Sockets) ConnectionHandler() http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    conn, err := s.upgrader.Upgrade(w, r, nil)

    if err != nil {
      http.Error(w, err.Error(), 500)
      return
    }

    s.lock.Lock()
    defer s.lock.Unlock()

    nameBytes := make([]byte, 32)
    if n, err := rand.Read(nameBytes); err != nil {
      log.Fatal(err)
    } else if n < len(nameBytes) {
      log.Fatalf("needed %d random bytes, got %d", len(nameBytes), n)
    }

    name := base64.StdEncoding.EncodeToString(nameBytes)

    s.connections[name] = conn

    // log.Printf("conn: %#v, name: `%s`", conn, name)

    go s.handleIncoming(name, conn)
  })
}
func sendError(conn *websocket.Conn, err error) error {
  return conn.WriteJSON(&server.Request{Error: err})
}
func sendMessage(conn *websocket.Conn, msg interface{}) error {
  return conn.WriteJSON(msg)
}
func (s *Sockets) handleIncoming(name string, conn *websocket.Conn) {
  log.Printf("incoming connection...")
  defer func() {
    s.lock.Lock()
    defer s.lock.Unlock()

    delete(s.connections, name)
    log.Printf("incoming connection closed.")
  }()

  for {
    _, msg, err := conn.ReadMessage()
    if err != nil {
      if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
        log.Printf("error: %v", err)
      }
      break
    }

    req := &server.Request{}
    if err = json.Unmarshal(msg, req); err != nil {
      log.Printf("message error: %v", err)

      if err = sendError(conn, err); err != nil {
        log.Printf("error: %v", err)
        break
      }
    }
    req.Requestor = name
    log.Printf("incoming request: %#v", req)

    s.Incoming <- req
  }
}
func (s *Sockets) Stop() {
  s.lock.Lock()
  defer s.lock.Unlock()

  for name, conn := range s.connections {
    conn.WriteMessage(websocket.CloseMessage, []byte{})
    delete(s.connections, name)
  }
}

func locker(lock sync.Locker, f func()) {
  lock.Lock()
  defer lock.Unlock()
  f()
}

/*
Notify sends a message to all listening sockets
*/
func (s *Sockets) Notify(req *server.Request) error {
  // ensure the req can be marshalled
  if _, err := json.Marshal(req); err != nil {
    log.Printf("json marshal error: %v", err)
    return err
  }

  toRemove := make(map[string]*websocket.Conn)

  if req.Error != nil {
    if req.Requestor != "" {
      var conn *websocket.Conn
      locker(s.lock, func() {
        conn = s.connections[req.Requestor]
      })
      if conn == nil {
        log.Printf("unknown connection: %s", req.Requestor)
      } else if err := sendMessage(conn, req); err != nil {
        log.Printf("error writing to socket: %v", err)
        toRemove[req.Requestor] = conn
      }
    }
  } else if req.Method == http.MethodGet && req.Requestor != "" {
    if conn := s.connections[req.Requestor]; conn == nil {
      log.Printf("connection not found: %s", req.Requestor)
    } else if err := sendMessage(conn, req); err != nil {
      toRemove[req.Requestor] = conn
    }
  } else {
    conns := make(map[string]*websocket.Conn)
    locker(s.lock, func() {
      for name, conn := range s.connections {
        conns[name] = conn
      }
    })

    for name, conn := range conns {
      if err := sendMessage(conn, req); err != nil {
        log.Printf("error writing to socket: %v", err)
        toRemove[name] = conn
      }
    }
  }

  locker(s.lock, func() {
    for name, conn := range toRemove {
      conn.WriteMessage(websocket.CloseMessage, []byte{})
      delete(s.connections, name)
    }
  })

  return nil
}
