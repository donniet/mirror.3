package main

import (
  "net/http"
  "io/ioutil"
  "log"
  server "github.com/donniet/mirror.3/serveJSON"
  "encoding/json"
  "strings"
  "time"
  "fmt"
  "flag"
)

type DateTime struct {
  Visible bool `json:"visible"`
}

type Weather struct {
  High float32 `json:"high"`
  Low float32 `json:"low"`
  Icon string `json:"icon"`
  Visible bool `json:"visible"`
}

type Stream struct {
  URL string `json:"url"`
  Name string `json:"name"`
  Visible bool `json:"visible"`
}

type Mirror struct {
  DateTime DateTime `json:"dateTime"`
  Weather Weather `json:"weather"`
  Streams []Stream `json:"streams"`
  Display Display `json:"display"`
  Faces Faces `json:"faces"`
}

type Faces struct {
  LastSeen *FaceDetected `json:"predicted,omitempty"`
  Threshold float32 `json:"threshold"`
}

type FaceDetected struct {
  Name string `json:"name"`
  Time time.Time `json:"time"`
  Probability float32 `json:"probability"`
}

type Display struct {
  PowerStatus string `json:"powerStatus"`
}

func (d *Display) MarshalJSON() ([]byte, error) {
  m := make(map[string]string)
  if d.PowerStatus == "" {
    m["powerStatus"] = "unknown"
  } else {
    m["powerStatus"] = d.PowerStatus
  }
  return json.Marshal(m)
}
func (d *Display) UnmarshalJSON(data []byte) error {
  var m map[string]string
  if err := json.Unmarshal(data, &m); err != nil {
    return err
  }
  power := m["powerStatus"]

  switch power {
  case "on":
  case "standby":
  case "unknown":
  default:
    return fmt.Errorf("unknown power value: '%s'", power)
  }

  d.PowerStatus = power
  return nil
}

type Saver struct {
  fileName string
  data interface{}
}
func (s Saver) Notify(req *server.Request) error {
  switch req.Method {
  case http.MethodPut:
  case http.MethodPost:
  case http.MethodDelete:
  default:
    return nil
  }

  if b, err := json.MarshalIndent(s.data, "", "\t"); err != nil {
    log.Fatal(err)
  } else if err = ioutil.WriteFile(s.fileName, b, 0660); err != nil {
    log.Fatal(err)
  }
  return nil
}

var (
  fileName = "state.json"
  addr = ":8080"
)

func init() {
  flag.StringVar(&fileName, "stateFile", fileName, "file name to save and restore state")
  flag.StringVar(&addr, "addr", addr, "address to listen on")
}

type State struct {
  Data interface{}
  watchers []server.Notifier
}
func (s *State) Watch(watcher server.Notifier) {
  s.watchers = append(s.watchers, watcher)
}
func (s *State) Unwatch(watcher server.Notifier) {
  found := len(s.watchers)
  for i, n := range s.watchers {
    if n == watcher {
      found = i
      break
    }
  }

  if found < len(s.watchers) {
    s.watchers[found] = s.watchers[len(s.watchers) - 1]
    s.watchers = s.watchers[:len(s.watchers) - 1]
  }
}
func (s *State) Request(req *server.Request) (*json.RawMessage, error) {
  req.Response, req.Error = server.ServeJSON(req, s.Data)

  for _, n := range s.watchers {
    go n.Notify(req)
  }

  return req.Response, req.Error
}

func main() {
  flag.Parse()
  log.SetFlags(log.Lshortfile | log.LstdFlags)

  d := &Mirror{}
  state := &State{Data: d}

  sockets := NewSockets()
  saver := &Saver{fileName, d}

  state.Watch(sockets)
  state.Watch(saver)

  if b, err := ioutil.ReadFile(fileName); err != nil {
    log.Fatal(err)
  } else if err = json.Unmarshal(b, d); err != nil {
    log.Fatal(err)
  }

  go func() {
    for req := range sockets.Incoming {
      req.Response, req.Error = server.ServeJSON(req, d);
      sockets.Notify(req)
    }
  }()

  mux := http.NewServeMux()
  mux.Handle("/", http.FileServer(http.Dir("client")))
  mux.Handle("/socket", sockets.ConnectionHandler())
  mux.Handle("/api/", http.StripPrefix("/api/", http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
    body, _ := ioutil.ReadAll(r.Body)

    req := &server.Request{
      Method: r.Method,
      Path: strings.Split(r.URL.Path, "/"),
      Body: (*json.RawMessage)(&body),
    }
    if len(body) == 0 {
      req.Body = nil
    }

    res, err := state.Request(req)
    if err != nil {
      http.Error(w, req.Error.Error(), 500)
      return
    }

    if res != nil {
      w.Write(*res)
    }
  })))

  log.Fatal(http.ListenAndServe(addr, mux))
}
