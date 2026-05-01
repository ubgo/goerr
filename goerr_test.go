package goerr

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNew_DefaultsBothMessages(t *testing.T) {
	e := New("oops")
	if e.Error() != "oops" {
		t.Errorf("Error: got %q, want %q", e.Error(), "oops")
	}
	if e.ErrorUser().Error() != "oops" {
		t.Errorf("ErrorUser: got %q, want %q", e.ErrorUser().Error(), "oops")
	}
}

func TestNew_WithMessageUser_Diverges(t *testing.T) {
	e := New("internal: db unreachable", WithMessageUser("Service unavailable, please retry"))
	if e.Error() != "internal: db unreachable" {
		t.Errorf("internal msg: got %q", e.Error())
	}
	if e.ErrorUser().Error() != "Service unavailable, please retry" {
		t.Errorf("user msg: got %q", e.ErrorUser().Error())
	}
}

func TestNewDefault_GenericMessage(t *testing.T) {
	e := NewDefault()
	if !strings.Contains(e.Error(), "error occurred") {
		t.Errorf("default message: got %q", e.Error())
	}
}

func TestNew_WithCode_Data_TraceID_Level(t *testing.T) {
	e := New("oops",
		WithCode("E_BAD"),
		WithData(map[string]int{"x": 1}),
		WithTraceID("trace-123"),
		WithLevel("warn"),
	)
	r := e.Response()
	if r.Code != "E_BAD" {
		t.Errorf("Code: %q", r.Code)
	}
	if r.TraceID != "trace-123" {
		t.Errorf("TraceID: %q", r.TraceID)
	}
	if r.Level != "warn" {
		t.Errorf("Level: %q", r.Level)
	}
	if r.Data == nil {
		t.Errorf("Data: nil")
	}
}

func TestSetMessage_SetMessageUser(t *testing.T) {
	e := New("orig")
	e.SetMessage("op-msg").SetMessageUser("user-msg")
	if e.Error() != "op-msg" {
		t.Errorf("Error: got %q", e.Error())
	}
	if e.ErrorUser().Error() != "user-msg" {
		t.Errorf("ErrorUser: got %q", e.ErrorUser().Error())
	}
}

func TestKV_FromOption_AndSetter(t *testing.T) {
	e := New("oops",
		WithKV("first", 1),
		WithKVMap(map[string]any{"second": 2, "third": "three"}),
	)
	e.SetKV("fourth", true)

	r := e.Response()
	if r.KV["first"] != 1 || r.KV["second"] != 2 || r.KV["third"] != "three" || r.KV["fourth"] != true {
		t.Errorf("KV: %v", r.KV)
	}
}

func TestToJSON_RoundTrip(t *testing.T) {
	e := New("oops",
		WithCode("E_BAD"),
		WithKV("k", "v"),
		WithMessageUser("user msg"),
	)
	r := e.Response()
	s := r.ToJSON()
	if s == "" {
		t.Fatalf("ToJSON: empty")
	}
	var got ErrorResult
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Message != "oops" || got.Code != "E_BAD" || got.MessageUser != "user msg" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestToJSON_UnmarshallableData(t *testing.T) {
	r := &ErrorResult{Data: make(chan int)}
	if r.ToJSON() != "" {
		t.Errorf("expected empty string on marshal failure")
	}
}

func TestCallbacks_FireOnError_AndErrorUser(t *testing.T) {
	t.Cleanup(ResetCallbacks)
	ResetCallbacks()

	var calls int32
	var lastResp *ErrorResult
	AddCallback(func(_ context.Context, r *ErrorResult) {
		atomic.AddInt32(&calls, 1)
		lastResp = r
	})

	e := New("oops", WithCode("E_X"))
	_ = e.Error()
	_ = e.ErrorUser().Error()

	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("calls: got %d, want 2", atomic.LoadInt32(&calls))
	}
	if lastResp == nil || lastResp.Code != "E_X" {
		t.Errorf("callback resp: %+v", lastResp)
	}
}

func TestCallbacks_WithCtx_Propagates(t *testing.T) {
	t.Cleanup(ResetCallbacks)
	ResetCallbacks()

	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("trace"), "abc")

	var seen any
	AddCallback(func(c context.Context, _ *ErrorResult) {
		seen = c.Value(ctxKey("trace"))
	})

	e := New("oops", WithCtx(ctx))
	_ = e.Error()

	if seen != "abc" {
		t.Errorf("ctx propagation: got %v, want %q", seen, "abc")
	}
}

func TestCallbacks_ConcurrentRegistration(t *testing.T) {
	t.Cleanup(ResetCallbacks)
	ResetCallbacks()

	var wg sync.WaitGroup
	var fired int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			AddCallback(func(_ context.Context, _ *ErrorResult) {
				atomic.AddInt32(&fired, 1)
			})
		}()
	}
	wg.Wait()

	_ = New("oops").Error()
	if atomic.LoadInt32(&fired) != 50 {
		t.Errorf("expected 50 callbacks fired, got %d", atomic.LoadInt32(&fired))
	}
}
