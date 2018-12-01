package serveJSON

import (
  "testing"
  "net/http"
  "encoding/json"
)

type TestStruct struct {
  Visible bool `json:"visible"`
  Integer int `json:"integer,omitempty"`
  Array []string `json:"array,omitempty"`
  NoTag bool
}

type TestStruct2 struct {
  Test *TestStruct `json:"test,omitempty"`
}

var tester *TestStruct
var tester2 *TestStruct2

func init() {
  tester = &TestStruct{
    Visible: false,
    Integer: 42,
    Array: []string{"zero", "one", "two", "three"},
    NoTag: false,
  }

  tester2 = &TestStruct2{
    Test: tester,
  }
}

func TestServeJSONGET(t *testing.T) {
  output, err := ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"visible"},
  }, tester)

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if (string)(*output) != "false" {
    t.Errorf("incorrect output, expected 'false', got '%s'", *output)
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"integer"},
  }, tester)

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if (string)(*output) != "42" {
    t.Errorf("incorrect output, expected '42', got '%s'", *output)
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"test", "visible"},
  }, tester2)

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if (string)(*output) != "false" {
    t.Errorf("incorrect output, expected 'false', got '%s'", *output)
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"array", "1"},
  }, tester)

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if (string)(*output) != "\"one\"" {
    t.Errorf("incorrect output, expected '\"one\"', got '%s'", *output)
  }

  _, err = ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"array", "one"},
  }, tester)

  if err == nil {
    t.Errorf("expected invalid index error, but got none")
  }

  _, err = ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"array", "10"},
  }, tester)

  if err == nil {
    t.Errorf("expected invalid index error, but got none")
  }

  _, err = ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"array", "-1"},
  }, tester)

  if err == nil {
    t.Errorf("expected invalid index error, but got none")
  }
}


func TestServeJSONGETExtra(t *testing.T) {
  _, err := ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"visible","extra"},
  }, tester)

  if err == nil {
    t.Errorf("expected non-leaf error, got none")
  }
}

func TestServeJSONGETName(t *testing.T) {
  output, err := ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"NoTag"},
  }, tester)

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if (string)(*output) != "false" {
    t.Errorf("incorrect output, expected 'false', got '%s'", *output)
  }
}

func TestServeJSONUnknownField(t *testing.T) {
  _, err := ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"unknown"},
  }, tester)

  if err == nil {
    t.Errorf("expected unknown field error, got none")
  }
}

func TestServeJSONPUT(t *testing.T) {
  body := []byte(`"four"`)
  bodyError := []byte(`42`)
  bodyJsonError := []byte(`nil`)

  var out string

  putTester := &TestStruct{
    Visible: false,
    Integer: 42,
    Array: []string{"zero", "one", "two", "three"},
    NoTag: false,
  }

  output, err := ServeJSON(&Request{
    Method: http.MethodPut,
    Body: (*json.RawMessage)(&body),
    Path: []string{"array"},
  }, putTester)

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if err = json.Unmarshal(*output, &out); err != nil {
    t.Errorf("output: `%s`, error: %v", *output, err)
  } else if out != "four" {
    t.Errorf("output: `%v`, expected `%v`", out, "\"four\"")
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodGet,
    Body: nil,
    Path: []string{"array"},
  }, putTester)

  var a []string

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if err := json.Unmarshal(*output, &a); err != nil {
    t.Errorf("error unmarshalling response: %v", err)
  } else if len(a) != 5 || a[4] != "four" {
    t.Errorf("expected a new array with 5 elements, but got: `%v`", a)
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodPut,
    Body: (*json.RawMessage)(&bodyError),
    Path: []string{"array"},
  }, putTester)

  if err == nil {
    t.Errorf("expected body error got none")
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodPut,
    Body: (*json.RawMessage)(&bodyJsonError),
    Path: []string{"array"},
  }, putTester)

  if err == nil {
    t.Errorf("expected body error got none")
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodPut,
    Body: (*json.RawMessage)(&body),
    Path: []string{"visible"},
  }, putTester)

  if err == nil {
    t.Errorf("expected error, non-slice got none")
  }
}

func TestServeJSONDelete(t *testing.T) {
  deleteTest := &TestStruct{
    Visible: false,
    Integer: 42,
    Array: []string{"zero", "one", "two", "three"},
    NoTag: false,
  }

  output, err := ServeJSON(&Request{
    Method: http.MethodDelete,
    Path: []string{"array", "0"},
    Body: nil,
  }, deleteTest)

  if err != nil {
    t.Error(err)
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodGet,
    Path: []string{"array"},
    Body: nil,
  }, deleteTest)

  var o []string

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if err = json.Unmarshal(*output, &o); err != nil {
    t.Error(err)
  } else if len(o) != 3 || o[0] != "one" || o[1] != "two" || o[2] != "three" {
    t.Errorf("expected `%v` got `%v`", []string{"one","two","three"}, o)
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodDelete,
    Path: []string{"visible"},
  }, deleteTest)

  if err == nil {
    t.Errorf("expected error, delete from non-slice, got none")
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodDelete,
    Path: []string{"array", "-1"},
  }, deleteTest)

  if err == nil {
    t.Errorf("expected error index out of bounds, got none")
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodDelete,
    Path: []string{"array", "4"},
  }, deleteTest)

  if err == nil {
    t.Errorf("expected error index out of bounds, got none")
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodDelete,
    Path: []string{"array", "a"},
  }, deleteTest)

  if err == nil {
    t.Errorf("expected error index out of bounds, got none")
  }
}


func TestServeJSONPOST(t *testing.T) {
  body := []byte(`true`)
  body_int := []byte(`54`)

  output, err := ServeJSON(&Request{
    Method: http.MethodPost,
    Body: (*json.RawMessage)(&body),
    Path: []string{"visible"},
  }, tester)

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if (string)(*output) != (string)(body) {
    t.Errorf("incorrect output, expected '%s', got '%s'", body, *output)
  }

  output, err = ServeJSON(&Request{
    Method: http.MethodPost,
    Body: (*json.RawMessage)(&body_int),
    Path: []string{"integer"},
  }, tester)

  if err != nil {
    t.Error(err)
  } else if output == nil {
    t.Errorf("output is nil")
  } else if (string)(*output) != (string)(body_int) {
    t.Errorf("expected '%s', got '%s'", body_int, *output)
  }
}


func TestServeJSONPOSTInvalidJSON(t *testing.T) {
  body := []byte(`truish`)

  _, err := ServeJSON(&Request{
    Method: http.MethodPost,
    Body: (*json.RawMessage)(&body),
    Path: []string{"visible"},
  }, tester)

  if err == nil {
    t.Errorf("expected JSON error when unmarshalling body, got none.")
  }
}

func TestServeJSONPOSTNonPointer(t *testing.T) {
  body := []byte(`true`)

  _, err := ServeJSON(&Request{
    Method: http.MethodPost,
    Body: (*json.RawMessage)(&body),
    Path: []string{"visible"},
  }, *tester)

  if err == nil {
    t.Errorf("expected non-set error, got none.")
  }
}

func TestServeJSONPOSTEmptyBody(t *testing.T) {
  _, err := ServeJSON(&Request{
    Method: http.MethodPost,
    Body: nil,
    Path: []string{"visible"},
  }, tester)

  if err == nil {
    t.Errorf("expected empty body error, got none.")
  }
}

func TestServeJSONPatch(t *testing.T) {
  _, err := ServeJSON(&Request{
    Method: http.MethodPatch,
    Body: nil,
    Path: []string{"visible"},
  }, tester)

  if err == nil {
    t.Errorf("expected invalid method error, got no such error")
  }
}
