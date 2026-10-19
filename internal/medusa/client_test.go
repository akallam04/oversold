package medusa

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.Handler, timeout time.Duration) (*Client, *Recorder) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	rec := NewRecorder("abc123", "2.21.2")
	c, err := NewClient(srv.URL, "pk_test", &http.Client{Timeout: timeout}, rec)
	if err != nil {
		t.Fatal(err)
	}
	return c, rec
}

func TestNewClientRefusesRemoteHost(t *testing.T) {
	if _, err := NewClient("http://shop.example.com", "pk", nil, NewRecorder("", "")); err == nil {
		t.Fatal("want error for remote host")
	}
}

func TestRecorderSuccess(t *testing.T) {
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-publishable-api-key"); got != "pk_test" {
			t.Errorf("publishable key header = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"region_id":"reg_1"}` {
			t.Errorf("request body = %s", body)
		}
		w.Write([]byte(`{"cart":{"id":"cart_1"}}`))
	}), time.Second)

	cart, err := c.CreateCart(context.Background(), "reg_1")
	if err != nil {
		t.Fatal(err)
	}
	if cart.ID != "cart_1" {
		t.Errorf("cart id = %q", cart.ID)
	}

	calls := rec.Calls()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	got := calls[0]
	if got.Method != "POST" || got.Path != "/store/carts" || got.Status != 200 || got.Step != "create_cart" {
		t.Errorf("call = %+v", got)
	}
	if string(got.RequestBody) != `{"region_id":"reg_1"}` || string(got.ResponseBody) != `{"cart":{"id":"cart_1"}}` {
		t.Errorf("bodies = %s / %s", got.RequestBody, got.ResponseBody)
	}
	if got.Error != "" || got.Timeout {
		t.Errorf("unexpected error fields: %q %v", got.Error, got.Timeout)
	}
	if got.OversoldCommit != "abc123" || got.MedusaVersion != "2.21.2" {
		t.Errorf("versions = %q %q", got.OversoldCommit, got.MedusaVersion)
	}
	if got.Start.Location() != time.UTC || got.End.Location() != time.UTC {
		t.Errorf("times not UTC: %v %v", got.Start, got.End)
	}
	if got.End.Before(got.Start.Time) {
		t.Errorf("end %v before start %v", got.End, got.Start)
	}
	var enc struct {
		Start string `json:"start"`
	}
	b, _ := json.Marshal(got)
	json.Unmarshal(b, &enc)
	if _, err := time.Parse(TimestampFormat, enc.Start); err != nil || len(enc.Start) != len(TimestampFormat) {
		t.Errorf("start %q not in %s: %v", enc.Start, TimestampFormat, err)
	}
	var back Call
	if err := json.Unmarshal(b, &back); err != nil || !back.Start.Equal(got.Start.Time) {
		t.Errorf("start did not round trip: %v %v", back.Start, err)
	}
}

func TestRecorderHTTPError(t *testing.T) {
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"type":"not_allowed","message":"Some variant does not have the required inventory"}`))
	}), time.Second)

	_, err := c.AddLineItem(context.Background(), "cart_1", "variant_1", 3)
	if err == nil {
		t.Fatal("want error")
	}
	calls := rec.Calls()
	if len(calls) != 1 {
		t.Fatalf("got %d calls", len(calls))
	}
	got := calls[0]
	if got.Status != 400 {
		t.Errorf("status = %d", got.Status)
	}
	want := "http 400 not_allowed: Some variant does not have the required inventory"
	if got.Error != want {
		t.Errorf("error = %q, want %q", got.Error, want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("returned error = %q", err)
	}
	if got.Timeout {
		t.Error("timeout set on a plain HTTP error")
	}
}

func TestRecorderTimeout(t *testing.T) {
	release := make(chan struct{})
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}), 50*time.Millisecond)
	defer close(release)

	_, err := c.CompleteCart(context.Background(), "cart_1")
	if err == nil {
		t.Fatal("want timeout error")
	}
	got := rec.Calls()[0]
	if !got.Timeout {
		t.Errorf("timeout not recorded: %+v", got)
	}
	if got.Status != 0 || got.Error == "" {
		t.Errorf("status %d error %q", got.Status, got.Error)
	}
	if d := got.End.Sub(got.Start.Time); d < 50*time.Millisecond {
		t.Errorf("duration %v shorter than the timeout", d)
	}
}

func TestRecorderNonJSONBody(t *testing.T) {
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("upstream down"))
	}), time.Second)

	c.ListRegions(context.Background())
	got := rec.Calls()[0]
	if string(got.ResponseBody) != `"upstream down"` {
		t.Errorf("response body = %s", got.ResponseBody)
	}
}

func TestCheckoutCallOrder(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		mu.Unlock()
		switch {
		case r.URL.Path == "/store/regions":
			w.Write([]byte(`{"regions":[{"id":"reg_eu","currency_code":"eur","countries":[{"iso_2":"de"}]},{"id":"reg_us","currency_code":"usd","countries":[{"iso_2":"us"}]}]}`))
		case r.URL.Path == "/store/products":
			w.Write([]byte(`{"products":[{"id":"prod_1","variants":[{"id":"var_piece","sku":"OVS-PIECE"},{"id":"var_two","sku":"OVS-TWOPACK"}]}]}`))
		case r.URL.Path == "/store/carts":
			w.Write([]byte(`{"cart":{"id":"cart_1"}}`))
		case r.URL.Path == "/store/shipping-options":
			w.Write([]byte(`{"shipping_options":[{"id":"so_1"}]}`))
		case r.URL.Path == "/store/payment-collections":
			w.Write([]byte(`{"payment_collection":{"id":"pay_col_1"}}`))
		case r.URL.Path == "/store/carts/cart_1/complete":
			w.Write([]byte(`{"type":"order","order":{"id":"order_1"}}`))
		default:
			w.Write([]byte(`{"cart":{"id":"cart_1"}}`))
		}
	}), time.Second)

	ctx := context.Background()
	lines, err := ParseLines("OVS-PIECE:2,OVS-TWOPACK:1")
	if err != nil {
		t.Fatal(err)
	}
	target, err := c.FindTarget(ctx, "usd", "us", []string{"OVS-PIECE", "OVS-TWOPACK"})
	if err != nil {
		t.Fatal(err)
	}
	if target.RegionID != "reg_us" {
		t.Errorf("region = %q", target.RegionID)
	}
	for i := range lines {
		lines[i].VariantID = target.Variants[lines[i].SKU]
	}
	res := c.Checkout(ctx, CheckoutInput{RegionID: target.RegionID, Lines: lines, Email: "a@example.com"})
	if !res.Completed || res.OrderID != "order_1" || res.CartID != "cart_1" {
		t.Fatalf("result = %+v", res)
	}

	want := []string{
		"GET /store/regions?limit=100",
		"GET /store/products?limit=100&fields=id,title,*variants",
		"POST /store/carts",
		"POST /store/carts/cart_1/line-items",
		"POST /store/carts/cart_1/line-items",
		"POST /store/carts/cart_1",
		"GET /store/shipping-options?cart_id=cart_1",
		"POST /store/carts/cart_1/shipping-methods",
		"POST /store/payment-collections",
		"POST /store/payment-collections/pay_col_1/payment-sessions",
		"POST /store/carts/cart_1/complete",
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Errorf("server saw:\n%s\nwant:\n%s", strings.Join(seen, "\n"), strings.Join(want, "\n"))
	}
	calls := rec.Calls()
	if len(calls) != len(want) {
		t.Fatalf("recorded %d calls, want %d", len(calls), len(want))
	}
	for i, call := range calls {
		if call.Index != i || call.Method+" "+call.Path != want[i] {
			t.Errorf("call %d = %d %s %s", i, call.Index, call.Method, call.Path)
		}
		if i > 0 && call.Start.Before(calls[i-1].End.Time) {
			t.Errorf("call %d started before call %d ended", i, i-1)
		}
	}
	if string(calls[3].RequestBody) != `{"quantity":2,"variant_id":"var_piece"}` {
		t.Errorf("first line item body = %s", calls[3].RequestBody)
	}
	if string(calls[9].RequestBody) != `{"provider_id":"pp_system_default"}` {
		t.Errorf("payment session body = %s", calls[9].RequestBody)
	}
}

func TestCheckoutStopsAtRejectedComplete(t *testing.T) {
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/store/shipping-options":
			w.Write([]byte(`{"shipping_options":[{"id":"so_1"}]}`))
		case "/store/payment-collections":
			w.Write([]byte(`{"payment_collection":{"id":"pay_col_1"}}`))
		case "/store/carts/cart_1/complete":
			w.Write([]byte(`{"type":"cart","cart":{"id":"cart_1"},"error":{"message":"not enough stock","name":"Error","type":"not_allowed"}}`))
		default:
			w.Write([]byte(`{"cart":{"id":"cart_1"}}`))
		}
	}), time.Second)

	res := c.Checkout(context.Background(), CheckoutInput{RegionID: "reg_us", Lines: []Line{{SKU: "OVS-PIECE", VariantID: "v", Quantity: 1}}})
	if res.Completed || res.FailedStep != "complete_cart" || !strings.Contains(res.Error, "not enough stock") {
		t.Errorf("result = %+v", res)
	}
	if n := len(rec.Calls()); n != 8 {
		t.Errorf("recorded %d calls, want 8", n)
	}
}

func TestParseLines(t *testing.T) {
	lines, err := ParseLines("OVS-PIECE:4, OVS-TWOPACK:2")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0] != (Line{SKU: "OVS-PIECE", Quantity: 4}) || lines[1] != (Line{SKU: "OVS-TWOPACK", Quantity: 2}) {
		t.Errorf("lines = %+v", lines)
	}
	for _, bad := range []string{"", "OVS-PIECE", "OVS-PIECE:0", "OVS-PIECE:x", ":2", "A:1,A:2"} {
		if _, err := ParseLines(bad); err == nil {
			t.Errorf("ParseLines(%q) = nil error", bad)
		}
	}
}
