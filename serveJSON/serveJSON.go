package serveJSON

import (
  "net/http"
  "encoding/json"
  "io/ioutil"
  "regexp"
  "fmt"
  "reflect"
  "sync"
  "strings"
  "strconv"
)

var typeCacheLock sync.Locker
var typeCache map[reflect.Type]map[string]reflect.StructField

var jsonTagParser *regexp.Regexp
var pathParser *regexp.Regexp

func init() {
  typeCacheLock = &sync.Mutex{}
  typeCache = make(map[reflect.Type]map[string]reflect.StructField)
  jsonTagParser = regexp.MustCompile("^([^,]+)(,.*)?$")
  pathParser = regexp.MustCompile("^/?([^/]+)(/.*)?$")
}

type HandlerJSON struct {
  Wrapped interface{}
}

func (h HandlerJSON) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  body, _ := ioutil.ReadAll(r.Body)

  res, err := ServeJSON(&Request{
    Method: r.Method,
    Path: strings.Split(r.URL.Path, "/")[1:], // the path always starts with a '/'
    Body: (*json.RawMessage)(&body),
  }, h.Wrapped)

  if err != nil {
    http.Error(w, err.Error(), 500)
    return
  }

  if res == nil {
    return
  }

  w.Write(*res)
}

type Request struct {
  Requestor string `json:"-"`
  Method string `json:"method,omitempty"`
  Path []string `json:"path,omitempty"`
  Error error `json:"error,omitempty"`
  Body *json.RawMessage `json:"body"`
  Response *json.RawMessage `json:"response,omitempty"`
}

type Notifier interface {
  Notify(req *Request) error
}

type Muxer struct {
  Clients []Notifier
}
func (m Muxer) Notify(req *Request) {
  for _, c := range m.Clients {
    go c.Notify(req)
  }
}

func locked(lock sync.Locker, f func()) {
  lock.Lock()
  defer lock.Unlock()
  f()
}

func ServeJSON(r *Request, face interface{}) (*json.RawMessage, error) {
  path := make([]string, len(r.Path))
  copy(path, r.Path)
  leaf := ""

  if len(path) == 1 && path[0] == "" {
    path = path[1:]
  }

  switch r.Method {
  case http.MethodGet:
  case http.MethodPost:
  case http.MethodPut:
  case http.MethodDelete:
    if len(path) == 0 {
      return nil, fmt.Errorf("unsuppored empty delete path")
    }
    leaf, path = path[len(path)-1], path[:len(path)-1]
  default:
    return nil, fmt.Errorf("unsuppored method '%s'", r.Method)
  }

  pv := reflect.ValueOf(face)

  if pv.Kind() != reflect.Ptr {
    return nil, fmt.Errorf("interface passed must be a ptr")
  }

  if pe, err := helper(path, pv); err != nil {
    return nil, err
  } else {
    switch r.Method {
    case http.MethodPost:
      if r.Body == nil || len(*r.Body) == 0 {
        return nil, fmt.Errorf("body is empty")
      }

      if err := json.Unmarshal(*r.Body, pe.Interface()); err != nil {
        return nil, err
      }
    case http.MethodPut:
      if inserted, err := putHelper(r.Body, pe); err != nil {
        return nil, err
      } else {
        pe = inserted
      }
    case http.MethodDelete:
      if err := deleteHelper(leaf, pe); err != nil {
        return nil, err
      }
      // don't return anything on delete
      return nil, nil
    }

    bytes, _ := json.Marshal(pe.Interface())
    return (*json.RawMessage)(&bytes), nil
  }
}

func deleteHelper(leaf string, pv reflect.Value) error {
  v := pv.Elem()
  t := v.Type()

  if t.Kind() != reflect.Slice {
    return fmt.Errorf("cannot delete from type of '%v'", t.Kind())
  }

  if i, err := strconv.Atoi(leaf); err != nil {
    return err
  } else {
    if i < 0 || i >= v.Len() {
      return fmt.Errorf("index out of bounds: %d", i)
    }

    dex := reflect.Copy(v.Slice(i, v.Len()), v.Slice(i+1, v.Len()))
    v.Set(v.Slice(0,i+dex))
  }
  return nil
}

func putHelper(body *json.RawMessage, pv reflect.Value) (reflect.Value, error) {
  v := pv.Elem() // pointer
  t := v.Type()

  if t.Kind() != reflect.Slice {
    return pv, fmt.Errorf("cannot put to type of '%v'", t.Kind())
  }

  elemType := t.Elem()

  item := reflect.New(elemType)

  // log.Printf("body: `%s`, item: %v", *body, item.Elem().Kind())

  if err := json.Unmarshal(*body, item.Interface()); err != nil {
    return pv, err
  }

  // log.Printf("item: `%v`", item.Elem().Interface())

  v.Set(reflect.Append(v, item.Elem()))

  // log.Printf("array: %v", pv.Interface())
  // pv.Set(v)

  return item.Elem(), nil
}

func helper(path []string, pv reflect.Value) (reflect.Value, error) {
  for len(path) > 0 {
    v := pv.Elem()
    t := v.Type()

    var err error

    switch t.Kind() {
    case reflect.Struct:
      pv, err = struct_helper(path[0], v, t)
    case reflect.Array:
      pv, err = array_helper(path[0], v)
    case reflect.Slice:
      pv, err = array_helper(path[0], v)
    // case reflect.Map:
    //   pv, err = map_helper(path[0], v)
    default:
      err = fmt.Errorf("path not found '%s', '%v'", strings.Join(path, "/"), t.Kind())
    }
    if err != nil {
      return pv, err
    }

    path = path[1:]

    if pv.Kind() != reflect.Ptr {
      pv = pv.Addr()
    }
  }
  return pv, nil
}

// func map_helper(index string, v reflect.Value) (reflect.Value, error) {
//   keyType := v.Type().Key()
//   valType := v.Type().Elem()
//
//   if keyType.Kind() != reflect.String {
//     return v, fmt.Errorf("only maps with key type of string are supported")
//   }
//
//
// }

func array_helper(index string, v reflect.Value) (reflect.Value, error) {
  var err error
  i := 0

  if i, err = strconv.Atoi(index); err != nil {
    return v, err
  }

  if i < 0 || i >= v.Len() {
    return v, fmt.Errorf("index '%d' out of bounds", i)
  }

  v = v.Index(i)

  return v, nil
}

func struct_helper(fieldName string, v reflect.Value, t reflect.Type) (reflect.Value, error) {
  var fields map[string]reflect.StructField
  ok := false

  var f reflect.StructField
  found := false

  locked(typeCacheLock, func() {
    if fields, ok = typeCache[t]; !ok {
      fields = make(map[string]reflect.StructField)
      typeCache[t] = fields
    }
    f, found = fields[fieldName]
  })

  inCache := found

  for i := 0; i < t.NumField() && !found; i++ {
    f = t.Field(i)

    if json, ok := f.Tag.Lookup("json"); ok {
      if parsed := jsonTagParser.FindStringSubmatch(json); len(parsed) > 1 {
        if parsed[1] == fieldName {
          found = true
          break
        } else if len(parsed) > 2 || parsed[1] != "" {
          // our fieldName doesn't match the named json field, don't check the field.Name
          continue
        }
        // else fallthrough
      }
    }

    if f.Name == fieldName {
      found = true
      break
    }
  }

  if !found {
    return v, fmt.Errorf("field name not found: '%s'", fieldName)
  }
  if !inCache {
    locked(typeCacheLock, func() {
      fields[fieldName] = f
    })
  }

  return v.FieldByIndex(f.Index), nil
}
