package medusa

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akallam04/oversold/internal/history"
)

func newTestClient(t *testing.T, h http.Handler, timeout time.Duration) (*Client, *history.Recorder) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	rec := history.NewRecorder(time.Now(), history.NewMasker("pk_test_key_value"))
	c, err := NewClient(srv.URL, "pk_test_key_value", &http.Client{Timeout: timeout}, rec)
	if err != nil {
		t.Fatal(err)
	}
	return c, rec
}

func TestNewClientRefusesRemoteHost(t *testing.T) {
	if _, err := NewClient("http://shop.example.com", "pk", nil, history.NewRecorder(time.Now(), nil)); err == nil {
		t.Fatal("want error for remote host")
	}
}

func TestRecordsSuccess(t *testing.T) {
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-publishable-api-key"); got != "pk_test_key_value" {
			t.Errorf("publishable key header = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"region_id":"reg_1"}` {
			t.Errorf("request body = %s", body)
		}
		w.Write([]byte(`{"cart":{"id":"cart_1"}}`))
	}), time.Second)

	cart, err := c.With(Labels{Cart: "main", Server: 2}).CreateCart(context.Background(), "reg_1")
	if err != nil {
		t.Fatal(err)
	}
	if cart.ID != "cart_1" {
		t.Errorf("cart id = %q", cart.ID)
	}
	ops := rec.Operations()
	if len(ops) != 1 {
		t.Fatalf("got %d operations, want 1", len(ops))
	}
	op := ops[0]
	if op.ID != 1 || op.Phase != history.PhaseSetup || op.Step != "create_cart" || op.Cart != "main" || op.Server != 2 {
		t.Errorf("labels = %+v", op)
	}
	if op.Method != "POST" || op.Path != "/store/carts" || op.Status != 200 || op.Outcome != history.OK {
		t.Errorf("op = %+v", op)
	}
	if string(op.RequestBody) != `{"region_id":"reg_1"}` || string(op.ResponseBody) != `{"cart":{"id":"cart_1"}}` {
		t.Errorf("bodies = %s / %s", op.RequestBody, op.ResponseBody)
	}
	if op.Error != "" || op.Timeout || op.StartNS < 0 || op.EndNS < op.StartNS || op.End.Before(op.Start.Time) {
		t.Errorf("timing or error fields = %+v", op)
	}
	if op.ConnReused == nil || *op.ConnReused {
		t.Errorf("first call should be on a new connection: %v", op.ConnReused)
	}
}

func TestRecordsHTTPError(t *testing.T) {
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"type":"not_allowed","message":"Some variant does not have the required inventory"}`))
	}), time.Second)

	_, err := c.AddLineItem(context.Background(), "cart_1", "variant_1", 3)
	if err == nil {
		t.Fatal("want error")
	}
	op := rec.Operations()[0]
	want := "http 400 not_allowed: Some variant does not have the required inventory"
	if op.Status != 400 || op.Outcome != history.Failed || op.Error != want || op.Timeout {
		t.Errorf("op = %+v", op)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("returned error = %q", err)
	}
}

func TestRecordsServerErrorAsUnknown(t *testing.T) {
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}), time.Second)

	_, op, err := c.CompleteCart(context.Background(), "cart_1")
	if err == nil || op.Outcome != history.Unknown || rec.Operations()[0].Outcome != history.Unknown {
		t.Errorf("err %v, op %+v", err, op)
	}
	if string(op.ResponseBody) != `"boom"` {
		t.Errorf("response body = %s", op.ResponseBody)
	}
}

func TestRecordsTimeoutAsUnknown(t *testing.T) {
	release := make(chan struct{})
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}), 50*time.Millisecond)
	defer close(release)

	if _, _, err := c.CompleteCart(context.Background(), "cart_1"); err == nil {
		t.Fatal("want timeout error")
	}
	op := rec.Operations()[0]
	if !op.Timeout || op.Outcome != history.Unknown || op.Status != 0 || op.Error == "" {
		t.Errorf("op = %+v", op)
	}
	if d := op.EndNS - op.StartNS; d < int64(50*time.Millisecond) {
		t.Errorf("duration %dns shorter than the timeout", d)
	}
}

func TestAdminLoginIsMasked(t *testing.T) {
	const password = "correct-horse-battery-staple"
	token := "eyJ" + strings.Repeat("h", 20) + ".eyJ" + strings.Repeat("p", 24) + "." + strings.Repeat("s", 22)
	var gotAuth string
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/user/emailpass":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), password) {
				t.Errorf("server did not get the password: %s", body)
			}
			w.Write([]byte(`{"token":"` + token + `"}`))
		default:
			gotAuth = r.Header.Get("Authorization")
			w.Write([]byte(`{"inventory_items":[]}`))
		}
	}), time.Second)

	ctx := context.Background()
	tok, err := c.AdminLogin(ctx, "admin@example.com", password)
	if err != nil || tok != token {
		t.Fatalf("login = %q, %v", tok, err)
	}
	if _, err := c.WithAdminToken(tok).ListInventoryItems(ctx, "WIDGET"); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer "+token {
		t.Errorf("admin call sent Authorization %q", gotAuth)
	}

	ops := rec.Operations()
	login := ops[0]
	if string(login.RequestBody) != `{"email":"admin@example.com","password":"[masked]"}` {
		t.Errorf("login request body = %s", login.RequestBody)
	}
	if string(login.ResponseBody) != `{"token":"[masked]"}` {
		t.Errorf("login response body = %s", login.ResponseBody)
	}

	path := filepath.Join(t.TempDir(), "run-001.json")
	h := &history.History{Schema: history.Schema, Operations: ops, HarnessError: "token was " + token}
	if err := history.Write(path, h, rec.Masker()); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	for _, leak := range []string{password, token, "pk_test_key_value"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("history file leaks %q", leak)
		}
	}
}

func TestCheckoutCallOrder(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	c, rec := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		mu.Unlock()
		switch r.URL.Path {
		case "/store/regions":
			w.Write([]byte(`{"regions":[{"id":"reg_eu","currency_code":"eur","countries":[{"iso_2":"de"}]},{"id":"reg_us","currency_code":"usd","countries":[{"iso_2":"us"}]}]}`))
		case "/store/products":
			w.Write([]byte(`{"products":[{"id":"prod_1","variants":[{"id":"var_piece","sku":"OVS-PIECE"},{"id":"var_two","sku":"OVS-TWOPACK"}]}]}`))
		case "/store/shipping-options":
			w.Write([]byte(`{"shipping_options":[{"id":"so_1"}]}`))
		case "/store/payment-collections":
			w.Write([]byte(`{"payment_collection":{"id":"pay_col_1"}}`))
		case "/store/carts/cart_1/complete":
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
	res := c.Checkout(ctx, CartInput{RegionID: target.RegionID, Lines: lines, Email: "a@example.com"})
	if !res.Completed || res.OrderID != "order_1" || res.CartID != "cart_1" || res.Outcome != history.OK {
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
	ops := rec.Operations()
	if len(ops) != len(want) {
		t.Fatalf("recorded %d operations, want %d", len(ops), len(want))
	}
	for i, op := range ops {
		if op.ID != i+1 || op.Method+" "+op.Path != want[i] || op.Outcome != history.OK {
			t.Errorf("op %d = %d %s %s %s", i, op.ID, op.Method, op.Path, op.Outcome)
		}
		if i > 0 && op.StartNS < ops[i-1].EndNS {
			t.Errorf("op %d started before op %d ended", i, i-1)
		}
		if i > 0 && (op.ConnReused == nil || !*op.ConnReused) {
			t.Errorf("op %d did not reuse the connection", i)
		}
	}
	if ops[len(ops)-1].Phase != history.PhaseRace {
		t.Errorf("complete phase = %q", ops[len(ops)-1].Phase)
	}
	if string(ops[3].RequestBody) != `{"quantity":2,"variant_id":"var_piece"}` {
		t.Errorf("first line item body = %s", ops[3].RequestBody)
	}
	if string(ops[9].RequestBody) != `{"provider_id":"pp_system_default"}` {
		t.Errorf("payment session body = %s", ops[9].RequestBody)
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

	res := c.Checkout(context.Background(), CartInput{RegionID: "reg_us", Lines: []Line{{SKU: "OVS-PIECE", VariantID: "v", Quantity: 1}}})
	if res.Completed || res.Outcome != history.Failed || res.FailedStep != history.StepComplete || !strings.Contains(res.Error, "not enough stock") {
		t.Errorf("result = %+v", res)
	}
	if n := len(rec.Operations()); n != 8 {
		t.Errorf("recorded %d operations, want 8", n)
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
